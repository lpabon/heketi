//
// Copyright (c) 2016 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubeexec

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	"k8s.io/kubernetes/pkg/fields"
	kubeletcmd "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
	"k8s.io/kubernetes/pkg/labels"
	certutil "k8s.io/kubernetes/pkg/util/cert"

	"github.com/lpabon/godbc"

	"github.com/heketi/heketi/executors/sshexec"
	"github.com/heketi/heketi/pkg/utils"
)

const (
	KubeGlusterFSPodLabelKey = "glusterfs-node"
	kubeServiceAccountDir    = "/var/run/secrets/kubernetes.io/serviceaccount/"
	kubeNameSpaceFile        = kubeServiceAccountDir + v1.ServiceAccountNamespaceKey
	kubeCAKeyFile            = kubeServiceAccountDir + v1.ServiceAccountRootCAKey
	kubeTokenFile            = kubeServiceAccountDir + v1.ServiceAccountTokenKey
)

type KubeExecutor struct {
	// Embed all sshexecutor functions
	sshexec.SshExecutor

	// save kube configuration
	config *KubeConfig
}

var (
	logger = utils.NewLogger("[kubeexec]", utils.LEVEL_DEBUG)
)

func setWithEnvVariables(config *KubeConfig) {
	var env string

	// Namespace / Project
	env = os.Getenv("HEKETI_KUBE_NAMESPACE")
	if "" != env {
		config.Namespace = env
	}

	// FSTAB
	env = os.Getenv("HEKETI_FSTAB")
	if "" != env {
		config.Fstab = env
	}

	// Snapshot Limit
	env = os.Getenv("HEKETI_SNAPSHOT_LIMIT")
	if "" != env {
		i, err := strconv.Atoi(env)
		if err == nil {
			config.SnapShotLimit = i
		}
	}

	// Determine if Heketi should communicate with Gluster
	// pods deployed by a DaemonSet
	env = os.Getenv("HEKETI_KUBE_GLUSTER_DAEMONSET")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.GlusterDaemonSet = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.GlusterDaemonSet = false
		}
	}

	// Use POD names
	env = os.Getenv("HEKETI_KUBE_USE_POD_NAMES")
	if "" != env {
		env = strings.ToLower(env)
		if env[0] == 'y' || env[0] == '1' {
			config.UsePodNames = true
		} else if env[0] == 'n' || env[0] == '0' {
			config.UsePodNames = false
		}
	}
}

func NewKubeExecutor(config *KubeConfig) (*KubeExecutor, error) {
	// Override configuration
	setWithEnvVariables(config)

	// Initialize
	k := &KubeExecutor{}
	k.config = config
	k.Throttlemap = make(map[string]chan bool)
	k.RemoteExecutor = k

	if k.config.Fstab == "" {
		k.Fstab = "/etc/fstab"
	} else {
		k.Fstab = config.Fstab
	}

	// Get namespace
	if k.config.Namespace == "" {
		var err error
		k.config.Namespace, err = k.readAllLinesFromFile(kubeNameSpaceFile)
		if err != nil {
			return nil, logger.LogError("Namespace must be provided in configuration: %v")
		}
	}

	// Show experimental settings
	if k.config.RebalanceOnExpansion {
		logger.Warning("Rebalance on volume expansion has been enabled.  This is an EXPERIMENTAL feature")
	}

	godbc.Ensure(k != nil)
	godbc.Ensure(k.Fstab != "")

	return k, nil
}

func (k *KubeExecutor) RemoteCommandExecute(host string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Throttle
	k.AccessConnection(host)
	defer k.FreeConnection(host)

	// Execute
	return k.ConnectAndExec(host,
		"pods",
		commands,
		timeoutMinutes)
}

func (k *KubeExecutor) ConnectAndExec(host, resource string,
	commands []string,
	timeoutMinutes int) ([]string, error) {

	// Used to return command output
	buffers := make([]string, len(commands))

	// Create a Kube client configuration
	clientConfig, err := InClusterConfig()
	if err != nil {
		return nil, err
	}

	// Get a client
	conn, err := client.New(clientConfig)
	if err != nil {
		logger.Err(err)
		return nil, fmt.Errorf("Unable to create a client connection")
	}

	// Get pod name
	var podName string
	if k.config.UsePodNames {
		podName = host
	} else if k.config.GlusterDaemonSet {
		podName, err = k.getPodNameFromDaemonSet(conn, host)
	} else {
		podName, err = k.getPodNameByLabel(conn, host)
	}
	if err != nil {
		return nil, err
	}

	for index, command := range commands {

		// Remove any whitespace
		command = strings.Trim(command, " ")

		// SUDO is *not* supported

		// Create REST command
		req := conn.RESTClient.Post().
			Resource(resource).
			Name(podName).
			Namespace(k.config.Namespace).
			SubResource("exec")
		req.VersionedParams(&api.PodExecOptions{
			Command: []string{"/bin/bash", "-c", command},
			Stdout:  true,
			Stderr:  true,
		}, api.ParameterCodec)

		// Create SPDY connection
		exec, err := remotecommand.NewExecutor(clientConfig, "POST", req.URL())
		if err != nil {
			logger.Err(err)
			return nil, fmt.Errorf("Unable to setup a session with %v", podName)
		}

		// Create a buffer to trap session output
		var b bytes.Buffer
		var berr bytes.Buffer

		// Excute command
		err = exec.Stream(remotecommand.StreamOptions{
			SupportedProtocols: kubeletcmd.SupportedStreamingProtocols,
			Stdout:             &b,
			Stderr:             &berr,
		})
		if err != nil {
			logger.LogError("Failed to run command [%v] on %v: Err[%v]: Stdout [%v]: Stderr [%v]",
				command, podName, err, b.String(), berr.String())
			return nil, fmt.Errorf("Unable to execute command on %v: %v", podName, berr.String())
		}
		logger.Debug("Host: %v Pod: %v Command: %v\nResult: %v", host, podName, command, b.String())
		buffers[index] = b.String()

	}

	return buffers, nil
}

func (k *KubeExecutor) RebalanceOnExpansion() bool {
	return k.config.RebalanceOnExpansion
}

func (k *KubeExecutor) SnapShotLimit() int {
	return k.config.SnapShotLimit
}

func (k *KubeExecutor) readAllLinesFromFile(filename string) (string, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", logger.LogError("Error reading %v file: %v", filename, err.Error())
	}
	return string(fileBytes), nil
}

func (k *KubeExecutor) getPodNameByLabel(conn *client.Client,
	host string) (string, error) {
	// 'host' is actually the value for the label with a key
	// of 'glusterid'
	selector, err := labels.Parse(KubeGlusterFSPodLabelKey + "==" + host)
	if err != nil {
		logger.Err(err)
		return "", logger.LogError("Unable to get pod with a matching label of %v==%v",
			KubeGlusterFSPodLabelKey, host)
	}

	// Get a list of pods
	pods, err := conn.Pods(k.config.Namespace).List(api.ListOptions{
		LabelSelector: selector,
		FieldSelector: fields.Everything(),
	})
	if err != nil {
		logger.Err(err)
		return "", fmt.Errorf("Failed to get list of pods")
	}

	numPods := len(pods.Items)
	if numPods == 0 {
		// No pods found with that label
		err := fmt.Errorf("No pods with the label '%v=%v' were found",
			KubeGlusterFSPodLabelKey, host)
		logger.Critical(err.Error())
		return "", err

	} else if numPods > 1 {
		// There are more than one pod with the same label
		err := fmt.Errorf("Found %v pods with the sharing the same label '%v=%v'",
			numPods, KubeGlusterFSPodLabelKey, host)
		logger.Critical(err.Error())
		return "", err
	}

	// Get pod name
	return pods.Items[0].ObjectMeta.Name, nil
}

func (k *KubeExecutor) getPodNameFromDaemonSet(conn *client.Client,
	host string) (string, error) {
	// 'host' is actually the value for the label with a key
	// of 'glusterid'
	selector, err := labels.Parse(KubeGlusterFSPodLabelKey)
	if err != nil {
		return "", logger.LogError("Unable to create selector of %v: %v",
			KubeGlusterFSPodLabelKey, err.Error())
	}

	// Get a list of pods
	pods, err := conn.Pods(k.config.Namespace).List(api.ListOptions{
		LabelSelector: selector,
		FieldSelector: fields.Everything(),
	})
	if err != nil {
		logger.Err(err)
		return "", logger.LogError("Failed to get list of pods")
	}

	// Go through the pods looking for the node
	var glusterPod string
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == host {
			glusterPod = pod.ObjectMeta.Name
		}
	}
	if glusterPod == "" {
		return "", logger.LogError("Unable to find a GlusterFS pod on host %v "+
			"with a label key %v", host, KubeGlusterFSPodLabelKey)
	}

	// Get pod name
	return glusterPod, nil
}

func InClusterConfig() (*restclient.Config, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, logger.LogError("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}

	token, err := ioutil.ReadFile(kubeTokenFile)
	if err != nil {
		return nil, err
	}
	tlsClientConfig := restclient.TLSClientConfig{}
	if _, err := certutil.NewPool(kubeCAKeyFile); err != nil {
		logger.LogError("Expected to load root CA config from %s, but got err: %v", kubeCAKeyFile, err)
	} else {
		tlsClientConfig.CAFile = kubeCAKeyFile
	}

	return &restclient.Config{
		// TODO: switch to using cluster DNS.
		Host:            "https://" + net.JoinHostPort(host, port),
		BearerToken:     string(token),
		TLSClientConfig: tlsClientConfig,
	}, nil
}
