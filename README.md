# Simple Kubernetes Cluster Capacity Finder


- Written in Golang.
- Program takes CPU/Memory resources for Kubernetes objects (say pods), required number of replicas as inputs and tells if you can really get them scheduled on your Kubernetes cluster !!
- It makes use of Kubernetes client-go API library under the hood.


### How to install/run

- Install go
- git clone https://github.com/AshutoshNirkhe/KubernetesClusterCapacity.git
- Create GOPATH directory e.g. "/opt/Kubernetes" : mkdir -p /opt/Kubernetes
- export GOPATH=/opt/Kubernetes
- cp -pR src /opt/Kubernetes
- cd /opt/Kubernetes/src/KubeAPI
- Download 'dep' (or yum install) if not present
- dep init
- Make sure ".kube/config" is present in home directory of your user or export KUBECONFIG path to wherever it is present.


### Usage flags
```
  -cpuLimits string
        CPU Limits either in cores(2) or milicores(500m) (default "200m")
  -cpuRequests string
        CPU Requests either in cores(1) or milicores(250m) (default "100m")
  -kubeconfig string
        (optional) absolute path to the kubeconfig file (default "${HOME}/.kube/config")
  -memLimits string
        Memory limits either in GB(2) or milicores(500mb) (default "200mb")
  -memRequests string
        Memory requests either in GB(1) or milicores(250mb) (default "100mb")
  -replicas string
        No of pod replicas (default "1")
```

### Sample run
- go run ClusterCapacity.go -cpuRequests=200m -cpuLimits=400m -memRequests=250mb -memLimits=500mb -replicas=10
==============================================================================================================

         Total possible replicas for the pod with required input specs : 1561
         So you can go ahead with deployment of 10 pod replicas in the Kubernetes cluster!!

==============================================================================================================

