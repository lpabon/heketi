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

	kubeapi "k8s.io/kubernetes/pkg/api/v1"
)

const (
	HeketiStoragePvFilename        = "heketi-storage-pv.json"
	HeketiStorageEndpointsFilename = "heketi-storage-endpoints.json"
	HeketiStorageSecretFilename    = "heketi-storage-secret.json"
	HeketiStorageEndpointName      = "heketi-storage-endpoints"
	HeketiStorageVolumeName        = "heketi-storage"
	HeketiStorageSecretName        = "heketi-storage-secret"
)

func init() {
	RootCmd.AddCommand(setupHeketiStorageCommand)
}

func saveJson(i interface{}, filename string) error {

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
		HeketiStorageVolumeName,
		HeketiStorageEndpointName)

	return saveJson(pv, HeketiStoragePvFilename)
}

func createHeketiStorageVolume(c *client.Client) error {

	// Show info
	fmt.Fprintln(stdout, "Creating volume heketi_storage...")

	// Create request
	req := &api.VolumeCreateRequest{}
	req.Size = 32
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

		return nil
	},
}
