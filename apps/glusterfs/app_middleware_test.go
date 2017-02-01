//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	fakeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5/fake"
	"k8s.io/kubernetes/pkg/client/restclient"
)

func init() {
	logger.SetLevel(utils.LEVEL_NOLOG)
}

func TestBackupToKubeSecretInvalidVerbs(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	incluster_count := 0
	defer tests.Patch(&inClusterConfig, func() (*restclient.Config, error) {
		incluster_count++
		return nil, nil
	}).Restore()

	// No backup when not POST or PUT
	r := &http.Request{
		Method: http.MethodGet,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 0)

	// Try again with another verb
	r = &http.Request{
		Method: http.MethodDelete,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 0)
}

func TestBackupToKubeSecretFailedClusterConfig(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	incluster_count := 0
	defer tests.Patch(&inClusterConfig, func() (*restclient.Config, error) {
		incluster_count++
		return nil, fmt.Errorf("TEST")
	}).Restore()

	config_count := 0
	defer tests.Patch(&newForConfig, func(c *restclient.Config) (clientset.Interface, error) {
		config_count++
		return nil, nil
	}).Restore()

	ns := "default"
	ns_count := 0
	defer tests.Patch(&getNamespace, func() (string, error) {
		ns_count++
		return ns, nil
	}).Restore()

	// Now try with POST verb
	r := &http.Request{
		Method: http.MethodPost,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 1)
	tests.Assert(t, config_count == 0)
	tests.Assert(t, ns_count == 0)

	// Try with PUT verb
	r = &http.Request{
		Method: http.MethodPut,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 2)
	tests.Assert(t, config_count == 0)
	tests.Assert(t, ns_count == 0)
}

func TestBackupToKubeSecretGoodBackup(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	incluster_count := 0
	defer tests.Patch(&inClusterConfig, func() (*restclient.Config, error) {
		incluster_count++
		return nil, nil
	}).Restore()

	config_count := 0
	defer tests.Patch(&newForConfig, func(c *restclient.Config) (clientset.Interface, error) {
		config_count++
		return fakeclientset.NewSimpleClientset(), nil
	}).Restore()

	ns := "default"
	ns_count := 0
	defer tests.Patch(&getNamespace, func() (string, error) {
		ns_count++
		return ns, nil
	}).Restore()

	// Now try with POST verb
	r := &http.Request{
		Method: http.MethodPost,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 1)
	tests.Assert(t, config_count == 1)
	tests.Assert(t, ns_count == 1)
}

func TestBackupToKubeSecretVerifyBackup(t *testing.T) {
	tmpfile := tests.Tempfile()
	defer os.Remove(tmpfile)

	// Create the app
	app := NewTestApp(tmpfile)
	defer app.Close()

	incluster_count := 0
	defer tests.Patch(&inClusterConfig, func() (*restclient.Config, error) {
		incluster_count++
		return nil, nil
	}).Restore()

	config_count := 0
	fakeclient := fakeclientset.NewSimpleClientset()
	defer tests.Patch(&newForConfig, func(c *restclient.Config) (clientset.Interface, error) {
		config_count++
		return fakeclient, nil
	}).Restore()

	ns := "default"
	ns_count := 0
	defer tests.Patch(&getNamespace, func() (string, error) {
		ns_count++
		return ns, nil
	}).Restore()

	// Add some content to the db
	c := NewClusterEntryFromRequest()
	c.NodeAdd("node_abc")
	c.NodeAdd("node_def")
	c.VolumeAdd("vol_abc")
	err := app.db.Update(func(tx *bolt.Tx) error {
		return c.Save(tx)
	})
	tests.Assert(t, err == nil)

	// Save to a secret
	r := &http.Request{
		Method: http.MethodPost,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 1)
	tests.Assert(t, config_count == 1)
	tests.Assert(t, ns_count == 1)

	// Get the secret
	secret, err := fakeclient.CoreV1().Secrets(ns).Get("heketi-db-backup")
	tests.Assert(t, err == nil)

	// Verify
	newdb := tests.Tempfile()
	defer os.Remove(newdb)
	err = ioutil.WriteFile(newdb, secret.Data["heketi.db"], 0644)
	tests.Assert(t, err == nil)

	// Load new app with backup
	app2 := NewTestApp(newdb)
	err = app2.db.View(func(tx *bolt.Tx) error {
		cluster, err := NewClusterEntryFromId(tx, c.Info.Id)
		tests.Assert(t, err == nil)
		tests.Assert(t, cluster != nil)

		return nil
	})
	tests.Assert(t, err == nil)
}
