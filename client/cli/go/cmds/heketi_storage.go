//
// Copyright (c) 2016 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package cmds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/kubernetes"
	"github.com/spf13/cobra"

	kube "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kubeapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/batch"
)

type KubeList struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Items      []interface{} `json:"items"`
}

const (
	HeketiStoragePvFilename        = "heketi-storage-pv.json"
	HeketiStorageEndpointsFilename = "heketi-storage-endpoints.json"
	HeketiStorageSecretFilename    = "heketi-storage-secret.json"
	HeketiStorageServiceFilename   = "heketi-storage-service.json"
	HeketiStoragePvcFilename       = "heketi-storage-pvc.json"
	HeketiStorageJobFilename       = "heketi-storage-job.json"
	HeketiStorageListFilename      = "heketi-storage.json"

	HeketiStorageJobName      = "heketi-storage-copy-job"
	HeketiStorageEndpointName = "heketi-storage-endpoints"
	HeketiStoragePvName       = "heketi-storage-pv"
	HeketiStorageSecretName   = "heketi-storage-secret"
	HeketiStoragePvcName      = "heketi-storage-pvc"

	HeketiStorageVolumeName    = "heketi_storage"
	HeketiStorageVolumeSize    = 32
	HeketiStorageVolumeSizeStr = "32Gi"

	HeketiStorageLabelKey   = "glustervol"
	HeketiStorageLabelValue = "heketi-storage"
)

var (
	list                      KubeList
	HeketiStorageJobContainer = "heketi/heketi:dev"
)

func init() {
	RootCmd.AddCommand(setupHeketiStorageCommand)
	list.APIVersion = "v1"
	list.Kind = "List"
	list.Items = make([]interface{}, 0)
}

func saveJson(i interface{}, filename string) error {

	// Global is ugly but good enough for now
	list.Items = append(list.Items, i)

	// Open File
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Marshal struct to JSON
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}

	// Save data to file
	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func createHeketiStorageVolumePv(c *client.Client,
	volume *api.VolumeInfoResponse) error {

	// Create PV
	fmt.Fprintf(stdout, "Saving %v\n", HeketiStoragePvFilename)
	pv := kubernetes.VolumeToPv(volume,
		HeketiStoragePvName,
		HeketiStorageEndpointName)
	pv.ObjectMeta.Labels = map[string]string{
		HeketiStorageLabelKey: HeketiStorageLabelValue,
	}

	return saveJson(pv, HeketiStoragePvFilename)
}

func createHeketiStorageVolume(c *client.Client) error {

	// Show info
	fmt.Fprintln(stdout, "Creating volume heketi_storage...")

	// Create request
	req := &api.VolumeCreateRequest{}
	req.Size = HeketiStorageVolumeSize
	req.Name = HeketiStorageVolumeName

	// Create volume
	volume, err := c.VolumeCreate(req)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Done\n")

	// Create PV
	err = createHeketiStorageVolumePv(c, volume)
	if err != nil {
		return nil
	}

	// Create endpoints
	err = createHeketiStorageEndpoints(c, volume)
	if err != nil {
		return nil
	}

	return nil
}

func createHeketiSecretFromDb(c *client.Client) error {
	var dbfile bytes.Buffer

	fmt.Fprintf(stdout, "Saving %v\n", HeketiStorageSecretFilename)

	// Save db
	err := c.BackupDb(&dbfile)
	if err != nil {
		return err
	}

	// Create Secret
	secret := &kubeapi.Secret{}
	secret.Kind = "Secret"
	secret.APIVersion = "v1"
	secret.ObjectMeta.Name = HeketiStorageSecretName
	secret.Data = make(map[string][]byte)
	secret.Data["heketi.db"] = dbfile.Bytes()

	return saveJson(secret, HeketiStorageSecretFilename)
}

func createHeketiEndpointService() error {
	fmt.Fprintf(stdout, "Saving %v\n", HeketiStorageServiceFilename)

	service := &kubeapi.Service{}
	service.Kind = "Service"
	service.APIVersion = "v1"
	service.ObjectMeta.Name = HeketiStorageEndpointName
	service.Spec.Ports = []kubeapi.ServicePort{
		kubeapi.ServicePort{
			Port: 1,
		},
	}

	return saveJson(service, HeketiStorageServiceFilename)
}

func createHeketiStorageEndpoints(c *client.Client,
	volume *api.VolumeInfoResponse) error {

	fmt.Fprintf(stdout, "Saving %v\n", HeketiStorageEndpointsFilename)

	endpoint := &kubeapi.Endpoints{}
	endpoint.Kind = "Endpoints"
	endpoint.APIVersion = "v1"
	endpoint.ObjectMeta.Name = HeketiStorageEndpointName
	endpoint.Subsets = make([]kubeapi.EndpointSubset, 1)

	// Get all node ids in the cluster with the volume
	cluster, err := c.ClusterInfo(volume.Cluster)
	if err != nil {
		return err
	}

	// Initialize slices
	endpoint.Subsets[0].Addresses = make([]kubeapi.EndpointAddress, len(cluster.Nodes))
	endpoint.Subsets[0].Ports = make([]kubeapi.EndpointPort, len(cluster.Nodes))

	// Save all nodes in the endpoints
	for n, nodeId := range cluster.Nodes {
		node, err := c.NodeInfo(nodeId)
		if err != nil {
			return err
		}

		// Determine if it is an IP address
		netIp := net.ParseIP(node.Hostnames.Storage[0])
		if netIp == nil {
			// It is not an IP, it is a hostname
			endpoint.Subsets[0].Addresses[n] = kubeapi.EndpointAddress{
				Hostname: node.Hostnames.Storage[0],
			}
		} else {
			// It is an IP
			endpoint.Subsets[0].Addresses[n] = kubeapi.EndpointAddress{
				IP: node.Hostnames.Storage[0],
			}
		}

		// Set to port 1
		endpoint.Subsets[0].Ports[n] = kubeapi.EndpointPort{
			Port: 1,
		}
	}

	return saveJson(endpoint, HeketiStorageEndpointsFilename)
}

func createHeketiStoragePvc() error {

	fmt.Fprintf(stdout, "Saving %v\n", HeketiStoragePvcFilename)

	pvc := &kubeapi.PersistentVolumeClaim{}
	pvc.Kind = "PersistentVolumeClaim"
	pvc.APIVersion = "v1"
	pvc.ObjectMeta.Name = HeketiStoragePvcName
	pvc.Spec.AccessModes = []kubeapi.PersistentVolumeAccessMode{
		kubeapi.ReadWriteMany,
	}
	pvc.Spec.Resources.Requests = kubeapi.ResourceList{
		kubeapi.ResourceStorage: resource.MustParse(HeketiStorageVolumeSizeStr),
	}

	/* Add this back when kubernetes supports it

	pvc.Spec.Selector = &unversioned.LabelSelector{
		MatchLabels: {
			HeketiStorageLabelKey: HeketiStorageLabelValue,
		},
	}
	*/

	return saveJson(pvc, HeketiStoragePvcFilename)
}

func createHeketiCopyJob() error {
	fmt.Fprintf(stdout, "Saving %v\n", HeketiStorageJobFilename)
	job := &batch.Job{}
	job.Kind = "Job"
	job.APIVersion = "extensions/v1beta1"
	job.ObjectMeta.Name = HeketiStorageJobName

	var (
		p int32 = 1
		c int32 = 1
	)
	job.Spec.Parallelism = &p
	job.Spec.Completions = &c
	job.Spec.Template.ObjectMeta.Name = HeketiStorageJobName
	job.Spec.Template.Spec.Volumes = []kube.Volume{
		kube.Volume{
			Name: HeketiStoragePvcName,
		},
		kube.Volume{
			Name: HeketiStorageSecretName,
		},
	}
	job.Spec.Template.Spec.Volumes[0].Glusterfs = &kube.GlusterfsVolumeSource{
		EndpointsName: HeketiStorageEndpointName,
		Path:          HeketiStorageVolumeName,
	}
	job.Spec.Template.Spec.Volumes[1].Secret = &kube.SecretVolumeSource{
		SecretName: HeketiStorageSecretName,
	}

	job.Spec.Template.Spec.Containers = []kube.Container{
		kube.Container{
			Name:  "heketi",
			Image: HeketiStorageJobContainer,
			Command: []string{
				"ls /",
				"ls /heketi",
				"ls /db",
			},
			VolumeMounts: []kube.VolumeMount{
				kube.VolumeMount{
					Name:      HeketiStoragePvcName,
					MountPath: "/heketi",
				},
				kube.VolumeMount{
					Name:      HeketiStorageSecretName,
					MountPath: "/db",
				},
			},
		},
	}
	job.Spec.Template.Spec.RestartPolicy = kube.RestartPolicyNever

	return saveJson(job, HeketiStorageJobFilename)
}

var setupHeketiStorageCommand = &cobra.Command{
	Use:   "setup-openshift-heketi-storage",
	Short: "Setup persistent storage for Heketi",
	Long: "Creates a dedicated GlusterFS volume for Heketi.\n" +
		"Once the volume is created, a set of Kubernetes/OpenShift\n" +
		"files are created to configure Heketi to use the newly create\n" +
		"GlusterFS volume.",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Create client
		c := client.NewClient(options.Url, options.User, options.Key)

		// Create volume
		err := createHeketiStorageVolume(c)
		if err != nil {
			return err
		}

		// Create secret
		err = createHeketiSecretFromDb(c)
		if err != nil {
			return nil
		}

		// Create service for the endpoints
		err = createHeketiEndpointService()
		if err != nil {
			return nil
		}

		// Create persistent volume claim
		err = createHeketiStoragePvc()
		if err != nil {
			return nil
		}

		// Create Job which copies db
		err = createHeketiCopyJob()
		if err != nil {
			return nil
		}

		// Save list
		fmt.Fprintf(stdout, "Saving %v\n", HeketiStorageListFilename)
		err = saveJson(list, HeketiStorageListFilename)
		if err != nil {
			return err
		}

		return nil
	},
}
