# Overview
Kubernetes templates for Heketi and Gluster. The following documentation is setup
to deploy the containers in Kubernetes.  It is not a full setup.  For full
documentation, please visit the Heketi wiki page.

# Usage

## Deploy Gluster

* Get node name by running:

```
$ kubectl get nodes
```

* Deploy gluster container onto specified node:

```
$ sed -e 's#<GLUSTERFS_NODE>#"..type your node here.."#' | kubectl creat -f -
```

> NOTE: The `""` are important around the node name

Repeat as needed.

## Deploy Heketi

* Create a service account for Heketi

```
$ kubectl create -f heketi-service-account.json
```

* Note the secret for the service account 

```
$ heketi_secret=$(kubectl get sa heketi-service-account -o="go-template" --template="{{range .secrets}}{{.name}}{{end}}")
```

* Determine your Kubernetes API HOST

```
$ context=$(kubectl config current-context)
$ cluster=$(kubectl config view -o jsonpath="{.contexts[?(@.name==\"$context\")].context.cluster}")
$ kubeapi=$(kubectl config view -o jsonpath="{.clusters[?(@.name==\"$cluster\")].cluster.server}")
```

* Deploy deploy-heketi

```
$ sed -e "s#<HEKETI_KUBE_NAMESPACE>#\"default\"#" \
      -e "s#<HEKETI_KUBE_SECRETNAME>#\"$heketi_secret\"#" \
      -e "s#<HEKETI_KUBE_APIHOST>#\"$kubeapi\"#" heketi-deployment.json | kubectl create -f -
```

