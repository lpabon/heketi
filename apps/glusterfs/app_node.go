//
// Copyright (c) 2015 The heketi Authors
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

package glusterfs

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/heketi/heketi/utils"
	"net/http"
)

func (a *App) NodeAdd(w http.ResponseWriter, r *http.Request) {
	var msg NodeAddRequest

	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w, "request unable to be parsed", 422)
		return
	}

	// Check information in JSON request
	if len(msg.Hostnames.Manage) == 0 {
		http.Error(w, "Manage hostname missing", http.StatusBadRequest)
		return
	}
	if len(msg.Hostnames.Storage) == 0 {
		http.Error(w, "Storage hostname missing", http.StatusBadRequest)
		return
	}

	// Check for correct values
	for _, name := range append(msg.Hostnames.Manage, msg.Hostnames.Storage...) {
		if name == "" {
			http.Error(w, "Hostname cannot be an empty string", http.StatusBadRequest)
			return
		}
	}

	// Get cluster and peer node
	var cluster *ClusterEntry
	var peer_node *NodeEntry
	err = a.db.View(func(tx *bolt.Tx) error {
		var err error
		cluster, err = NewClusterEntryFromId(tx, msg.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Get a node in the cluster to execute the Gluster peer command
		peer_node, err = cluster.PeerNode(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logger.Err(err)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Create a node entry
	node := NewNodeEntryFromRequest(&msg)

	// Add node
	logger.Info("Adding node %v", node.ManageHostName())
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (string, error) {

		// Peer probe if there is at least one other node
		// TODO: What happens if the peer_node is not responding.. we need to choose another.
		if peer_node != nil {
			err := a.executor.PeerProbe(peer_node.ManageHostName(), node.ManageHostName())
			if err != nil {
				return "", err
			}
		}

		// Add node entry into the db
		err = a.db.Update(func(tx *bolt.Tx) error {
			cluster, err := NewClusterEntryFromId(tx, msg.ClusterId)
			if err == ErrNotFound {
				http.Error(w, "Cluster id does not exist", http.StatusNotFound)
				return err
			} else if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			// Add node to cluster
			cluster.NodeAdd(node.Info.Id)

			// Save cluster
			err = cluster.Save(tx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			// Set node as online
			err = node.SetState(ENTRY_STATE_READY)
			if err != nil {
				return err
			}

			// Save node
			err = node.Save(tx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}
		logger.Info("Added node " + node.Info.Id)
		return "/nodes/" + node.Info.Id, nil
	})
}

func (a *App) NodeInfo(w http.ResponseWriter, r *http.Request) {

	// Get node id from URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get Node information
	var info *NodeInfoResponse
	err := a.db.View(func(tx *bolt.Tx) error {
		entry, err := NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, "Id not found", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		info, err = entry.NewInfoReponse(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	// Write msg
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		panic(err)
	}

}

func (a *App) NodeDelete(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	// Get node info
	var (
		peer_node, node *NodeEntry
		cluster         *ClusterEntry
	)
	err := a.db.Update(func(tx *bolt.Tx) error {

		// Access node entry
		var err error
		node, err = NewNodeEntryFromId(tx, id)
		if err == ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Check the node can be deleted
		if !node.IsDeleteOk() {
			http.Error(w, ErrConflict.Error(), http.StatusConflict)
			return ErrConflict
		}

		// Set the state accordingly
		err = node.SetState(ENTRY_STATE_DELETING)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return err
		}

		// Save state
		err = node.Save(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Access cluster information and peer node
		cluster, err = NewClusterEntryFromId(tx, node.Info.ClusterId)
		if err == ErrNotFound {
			http.Error(w, "Cluster id does not exist", http.StatusNotFound)
			return err
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		// Get a node in the cluster to execute the Gluster peer command
		peer_node, err = cluster.PeerNode(tx)
		if err != nil {
			logger.Err(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	// Delete node asynchronously
	logger.Info("Deleting node %v [%v]", node.ManageHostName(), node.Info.Id)
	a.asyncManager.AsyncHttpRedirectFunc(w, r, func() (s string, e error) {

		// If we have detected an error, move the node out of the deleting
		// state into an online state.
		defer func() {
			if e != nil {
				NodeSetStateReady(a.db, node.Info.Id)
			}
		}()

		// Remove from trusted pool
		if peer_node != nil {
			err := a.executor.PeerDetach(peer_node.ManageHostName(), node.ManageHostName())
			if err != nil {
				return "", err
			}
		}

		// Remove from db
		err = a.db.Update(func(tx *bolt.Tx) error {

			// Get Cluster
			cluster, err := NewClusterEntryFromId(tx, node.Info.ClusterId)
			if err == ErrNotFound {
				logger.Critical("Cluster id %v is expected be in db. Pointed to by node %v",
					node.Info.ClusterId,
					node.Info.Id)
				return err
			} else if err != nil {
				logger.Err(err)
				return err
			}

			// Remove node from list of nodes in the cluster
			cluster.NodeDelete(node.Info.Id)

			// Save cluster
			err = cluster.Save(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			// Delete node from db
			err = node.Delete(tx)
			if err != nil {
				logger.Err(err)
				return err
			}

			return nil

		})
		if err != nil {
			return "", err
		}
		// Show that the key has been deleted
		logger.Info("Deleted node [%s]", id)

		return "", nil

	})
}
