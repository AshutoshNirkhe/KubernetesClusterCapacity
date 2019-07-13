# KubernetesClusterCapacity
Simple Kubernetes Cluster Capacity Finder in Golang.


### How to run 

- git clone https://github.com/AshutoshNirkhe/KubernetesClusterCapacity.git
- Create GOPATH directory e.g. "/opt/Kubernetes" : mkdir -p /opt/Kubernetes
- export GOPATH=/opt/Kubernetes
- cp -pR src /opt/Kubernetes
- cd /opt/Kubernetes/src/KubeAPI
- Download 'dep' (or yum install) if not present
- dep init
- Make sure ".kube/config" is present in home directory of your user or export KUBECONFIG path to wherever it is present.

### Sample run -
- go run ClusterCapacity.go -cpuRequests=200m -cpuLimits=400m -memRequests=250mb -memLimits=500mb -replicas=10
'''===============================================================================================================================

         Total possible replicas for the pod with required input specs : 1561
         So you can go ahead with deployment of 10 pod replicas in the Kubernetes cluster!!

   ==============================================================================================================================='''
