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
	"bytes"
	"net/http"

	"github.com/boltdb/bolt"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"

	"github.com/heketi/heketi/pkg/kubernetes"

	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/v1"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	"k8s.io/kubernetes/pkg/client/restclient"
)

var (
	inClusterConfig = restclient.InClusterConfig
	newForConfig    = func(c *restclient.Config) (clientset.Interface, error) {
		return clientset.NewForConfig(c)
	}
	getNamespace = kubernetes.GetNamespace
)

// Authorization function
func (a *App) Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	// Value saved by the JWT middleware.
	data := context.Get(r, "jwt")

	// Need to change from interface{} to the jwt.Token type
	token := data.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)

	// Check access
	if "user" == claims["iss"] && r.URL.Path != "/volumes" {
		http.Error(w, "Administrator access required", http.StatusUnauthorized)
		return
	}

	// Everything is clean
	next(w, r)
}

// Authorization function
func (a *App) BackupToKubernetesSecret(
	w http.ResponseWriter,
	r *http.Request,
	next http.HandlerFunc) {

	// Call the next middleware first
	next(w, r)

	// Only backup for POST and PUT
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return
	}

	// Get Kubernetes configuration
	kubeConfig, err := inClusterConfig()
	if err != nil {
		logger.LogError("Unable to get Kubernetes configuration")
		return
	}

	// Get clientset
	c, err := newForConfig(kubeConfig)
	if err != nil {
		logger.Err(err)
		return
	}

	// Get namespace
	ns, err := getNamespace()
	if err != nil {
		logger.Err(err)
		return
	}

	// Get a backup
	var backup bytes.Buffer
	err = a.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(&backup)
		return err
	})
	if err != nil {
		logger.Err(err)
		return
	}

	// Create client for secrets
	secrets := c.CoreV1().Secrets(ns)
	if err != nil {
		logger.Err(err)
		return
	}

	// Create a secret with backup
	secret := &v1.Secret{}
	secret.Kind = "Secret"
	secret.Namespace = ns
	secret.APIVersion = "v1"
	secret.ObjectMeta.Name = "heketi-db-backup"
	secret.ObjectMeta.Labels = map[string]string{
		"heketi":    "db",
		"glusterfs": "heketi-db",
	}
	secret.Data = map[string][]byte{
		"heketi.db": backup.Bytes(),
	}

	// Submit secret
	_, err = secrets.Create(secret)
	if apierrors.IsAlreadyExists(err) {
		// It already exists, so just update it instead
		_, err = secrets.Update(secret)
		if err != nil {
			logger.LogError("Unable to save database to secret: %v", err)
			return
		}
		logger.Info("Backup updated successfully")
	} else if err != nil {
		logger.LogError("Unable to create database secret: %v", err)
		return
	}
	logger.Info("Backup successful")
}
