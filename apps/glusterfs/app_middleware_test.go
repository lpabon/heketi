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
	"net/http"
	"os"
	"testing"

	"github.com/heketi/heketi/pkg/utils"
	"github.com/heketi/tests"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	fakeclientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5/fake"
	"k8s.io/kubernetes/pkg/client/restclient"
)

func init() {
	logger.SetLevel(utils.LEVEL_DEBUG)
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
	defer tests.Patch(&newForConfig, func(c *restclient.Config) (*clientset.Clientset, error) {
		config_count++
		return nil, nil
	})

	// Now try with POST verb
	r := &http.Request{
		Method: http.MethodPost,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 1)
	tests.Assert(t, config_count == 0)

	// Try with PUT verb
	r = &http.Request{
		Method: http.MethodPut,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 2)
	tests.Assert(t, config_count == 0)
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

	// Now try with POST verb
	r := &http.Request{
		Method: http.MethodPost,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 1)
	tests.Assert(t, config_count == 0)

	// Try with PUT verb
	r = &http.Request{
		Method: http.MethodPut,
	}
	app.BackupToKubernetesSecret(nil, r, func(w http.ResponseWriter, r *http.Request) {})
	tests.Assert(t, incluster_count == 2)
	tests.Assert(t, config_count == 0)
}
