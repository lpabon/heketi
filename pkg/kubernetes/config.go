//
// Copyright (c) 2017 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kubernetes

import (
	"io/ioutil"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
)

const (
	KubeServiceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount/"
	KubeNameSpaceFile     = KubeServiceAccountDir + v1.ServiceAccountNamespaceKey
)

func GetNamespace() (string, error) {
	data, err := ioutil.ReadFile(KubeNameSpaceFile)
	if err != nil {
		return "", nil
	}
	if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
		return ns, nil
	}
	return api.NamespaceDefault, nil
}
