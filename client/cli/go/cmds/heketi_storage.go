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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	client "github.com/heketi/heketi/client/api/go-client"
	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/kubernetes"
	"github.com/spf13/cobra"

	kubeapi "k8s.io/kubernetes/pkg/api/v1"
)

func init() {
	RootCmd.AddCommand(setupHeketiStorageCommand)
}

func createHeketiStorageVolume(c *client.Client) error {
	// Show info
	fmt.Fprintf(stdout, "Creating volume heketi_storage...")

	// Create request
	req := &api.VolumeCreateRequest{}
	req.Size = 32
	req.Name = "heketi_storage"

	// Create volume
	volume, err := c.VolumeCreate(req)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Done\n")

	// Create PV
	pv := kubernetes.VolumeToPv(volume,
		"heketi-storage",
		"glusterfs-cluster")
	data, err := json.MarshalIndent(pv, "", "  ")
	if err != nil {
		return err
	}

	// Save PV
	f, err := os.Create("heketi-storage-pv.json")
	if err != nil {
		return err
	}
	f.Write(data)
	f.Close()

	return nil
}

func createHeketiSecretFromDb(c *client.Client) error {
	var dbfile bytes.Buffer

	// Save db
	err := c.BackupDb(&dbfile)
	if err != nil {
		return err
	}

	// Encode db
	encoded := base64.StdEncoding.EncodeToString(dbfile.Bytes())

	// Create Secret
	secret := &kubeapi.Secret{}
	secret.Kind = "Secret"
	secret.APIVersion = "v1"
	secret.ObjectMeta.Name = "heketidb"
	secret.Data = make(map[string][]byte)
	secret.Data["heketi.db"] = []byte(encoded)

	// Save json file
	f, err := os.Create("heketi-storage-secret.json")
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(secret, "", " ")
	if err != nil {
		return err
	}

	// Write to file
	f.Write(data)
	f.WriteString(encoded)
	f.Close()

	return nil
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
