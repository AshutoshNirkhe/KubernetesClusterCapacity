/* Overall logic and flow of program

Inputs : kubeconfig, cpuRequests, cpuLimits, memRequests, memLimits, replicas (needed for pods)

Output : Total possible replicas for pod of given resource requirements.

Code :
1. Verify all inputs for correctness, assign default values wherever possible, throw errors otherwise.
2. Create a kubernetes clientset object from kubeconfig configuration.
3. 'getHealthyNodes' function retrieves all the nodes which doesn't have any memory, disk, cpu pressure etc.
   In addition, it will also return totalAllocatableCPU, totalAllocatableMemory and totalAllocatablePods(110 by default).
4.a. For each health node, call 'getNonTerminatedPodsForNode' to get list of all non-terminated pods (pods whose status is no Pending, Succedded, Failed or Unknown)
  b. For all pods (on a given node), call 'getPodCPUMemoryRequestsLimits' to calculate the sum of memory, cpu requests and limits of all the containers running in these pods.
	 This will basically give the total summation of cpuRequestsUsed, cpuLimitsUsed, memoryRequestsUsed, memoryLimitsUsed on a given node.
  c. Now if totalCpuRequestsUsed > totalAllocatableCPU, the node is already full and hence no replicas can be schedulded.
	 Else, maxPossibleCPUReplicas (based on CPU requests alone) = ( totalAllocatableCPU - totalCpuRequestsUsed ) / cpuRequests
  d. Similarly maxPossibleMemoryReplicas (based on Memory requests alone) = ( totalAllocatableMemory - totalMemoryRequestsUsed ) / memRequests
  e. Find minimum of maxPossibleCPUReplicas and maxPossibleMemoryReplicas. This is the maxReplicasPerNode that can be scheduled (on this node).
5. Repeat step 4 for all other nodes. Summation of maxReplicasPerNode for all the nodes will be the totalMaxReplicas that can be scheduled on the kubernetes cluster.

*/

package main

import (
	"bytefmt"
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	cpuRquestsStr := flag.String("cpuRequests", "100m", "CPU Requests either in cores(1) or milicores(250m)")
	cpuLimitsStr := flag.String("cpuLimits", "200m", "CPU Limits either in cores(2) or milicores(500m)")
	memRequestsStr := flag.String("memRequests", "100mb", "Memory requests either in GB(1) or megabytes(250mb)")
	memLimitsStr := flag.String("memLimits", "200mb", "Memory limits either in GB(2) or megabytes(500mb)")
	replicasStr := flag.String("replicas", "1", "No of pod replicas")
	flag.Parse()

	cpuRequests := convertCPUToMilis(*cpuRquestsStr)
	cpuLimits := convertCPUToMilis(*cpuLimitsStr)

	memRequests, memReqErr := bytefmt.ToBytes(*memRequestsStr)
	if memReqErr != nil {
		fmt.Printf("Invalid input...exiting")
	}

	memLimits, memLimErr := bytefmt.ToBytes(*memLimitsStr)
	if memLimErr != nil {
		fmt.Printf("Invalid input...exiting")
	}

	replicas, numerr := strconv.Atoi(*replicasStr)
	if numerr != nil {
		fmt.Printf("Invalid input for replicas...exiting")
		replicas = 0
	}

	fmt.Printf("\nCPU limits, requests, Memory limits, requests and replicas parsed from input : %v %v %v %v %v\n", cpuLimits, cpuRequests, memLimits, memRequests, replicas)

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	nodes, nodesCPU, nodesMemory, nodeAllocatablePods := getHealthyNodes(clientset)

	//fmt.Printf("\nHealthy nodes are : %v\n\n", nodes)
	//fmt.Printf("\nHealthy nodes CPU are : %v\n\n", nodesCPU)
	//fmt.Printf("\nHealthy nodes Memory are : %v\n\n", nodesMemory)
	//fmt.Printf("\nHealthy nodes Max Allocatable pods : %v\n\n", nodeAllocatablePods)

	totalPossibleMaxReplicas := 0
	possibleMaxCPUReplicas := 0
	possibleMaxMemoryReplicas := 0

	for index, node := range nodes {
		pods, namespaces := getNonTerminatedPodsForNode(clientset, node)
		fmt.Printf("\n%v - ", node)
		fmt.Printf("Current non-terminated pods : %d", len(pods))
		cpuLimitsMiliTotal, cpuRequestsMiliTotal, memoryLimitsBytesTotal, memoryRequestsBytesTotal := getPodCPUMemoryRequestsLimits(clientset, pods, namespaces)
		fmt.Printf("\nSum of CPU Limits, Requests and Memory Limits, Requests for all pods : %v %v %v %v", cpuLimitsMiliTotal, cpuRequestsMiliTotal, memoryLimitsBytesTotal, memoryRequestsBytesTotal)
		fmt.Printf("\nTotal allocatbale CPU and Memory : %v, %v", nodesCPU[index], nodesMemory[index])

		cpuRequestUsedPercent := float64(cpuRequestsMiliTotal) * 100 / float64(nodesCPU[index])
		memoryRequestUsedPercent := float64(memoryRequestsBytesTotal) * 100 / float64(nodesMemory[index])
		cpuLimitUsedPercent := float64(cpuLimitsMiliTotal) * 100 / float64(nodesCPU[index])
		memoryLimitUsedPercent := float64(memoryLimitsBytesTotal) * 100 / float64(nodesMemory[index])
		fmt.Printf("\nCPU Limits, Requests and Memory Limits, Requests used percentage till now : %.2f %.2f %.2f %.2f", cpuLimitUsedPercent, cpuRequestUsedPercent, memoryLimitUsedPercent, memoryRequestUsedPercent)

		if nodesCPU[index] <= cpuRequestsMiliTotal {
			//fmt.Printf("\nCPU requests full..can't satisfy the requests")
			possibleMaxCPUReplicas = 0			
		} else {
			possibleMaxCPUReplicas = int((nodesCPU[index] - cpuRequestsMiliTotal) / cpuRequests)
		}
		if nodesMemory[index] <= memoryRequestsBytesTotal {
			//fmt.Printf("\nMemory requests full..can't satisfy the requests")
			possibleMaxMemoryReplicas = 0
		} else {
			possibleMaxMemoryReplicas = int((nodesMemory[index] - memoryRequestsBytesTotal) / memRequests)
		}

		//fmt.Printf("\nPossible replicas with CPU and Memory requests: %v %v", possibleMaxCPUReplicas, possibleMaxMemoryReplicas)
		maxReplicas := findMin(possibleMaxCPUReplicas, possibleMaxMemoryReplicas)
		if maxReplicas >= nodeAllocatablePods[index] {
			maxReplicas = nodeAllocatablePods[index] - len(pods)
		}
		fmt.Printf("\nMax replicas : %v\n", maxReplicas)
		totalPossibleMaxReplicas = totalPossibleMaxReplicas + maxReplicas

	}

	fmt.Println("==============================================================================================================")
	fmt.Printf("\n\t Total possible replicas for the pod with required input specs : %v", totalPossibleMaxReplicas)
	if totalPossibleMaxReplicas >= replicas {
		fmt.Printf("\n\t So you can go ahead with deployment of %v pod replicas in the Kubernetes cluster!!\n\n", replicas)
	} else {
		fmt.Printf("\n\t Unfortunately Kubernetes cluster can't scehdule %v replicas. Please try again by reducing the number of replicas or/and cpu/memory resource requests. Exiting!!\n\n", replicas)
	}
	fmt.Println("==============================================================================================================")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func userInput() string {

	var input string
	_, inputerr := fmt.Scan(&input)
	if inputerr != nil {
		fmt.Printf("\nProblem getting user input.\n")
		os.Exit(1)
	}
	return input
}

func findMin(x int, y int) int {
	if x <= y {
		return x
	}
	return y
}

func getHealthyNodes(clientset *kubernetes.Clientset) ([]string, []uint64, []int64, []int) {
	healthyNodes := make([]string, 0, 3)
	nodesCPU := make([]uint64, 0, 3)
	nodesMemory := make([]int64, 0, 3)
	nodeAllocatablePods := make([]int, 0, 3)

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	noOfNodes := len(nodes.Items)
	//fmt.Printf("\nThere are total %d nodes in the cluster\n\n", noOfNodes)

	for i := 0; i < noOfNodes; i++ {

		var flagHealthy bool = true
		node := nodes.Items[i].Name

		nodeDetails, err := clientset.CoreV1().Nodes().Get(node, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}

		//Get rid of master nodes.
		isNode := nodeDetails.Labels["node-role.kubernetes.io/node"]
		if isNode != "true" {
			//fmt.Printf("%s is not a worker node..skipping\n", node)
			continue
		}

		nodeAllocatableCPUDetails := nodeDetails.Status.Allocatable["cpu"]
		nodeCPUAllocatable := convertCPUToMilis(nodeAllocatableCPUDetails.String())

		nodeAllocatableMemoryDetails := nodeDetails.Status.Allocatable["memory"]
		//fmt.Printf("\nNode Allocatable memory : %v\n", nodeAllocatableMemoryDetails.String())

		nodeMemoryAllocatable, byteserr := bytefmt.ToBytes(nodeAllocatableMemoryDetails.String())
		if byteserr != nil {
			//fmt.Printf("\nError converting to Bytes\n")
			nodeMemoryAllocatable = 0
		}

		allocatablePods := int(nodeDetails.Status.Allocatable.Pods().Value())
		//allocatablePods := allocatablePodsDetails.String()
		//fmt.Printf("\nNode %v : CPU - %v , Memory - %v , Pods - %v\n", node, nodeCPUAllocatable, nodeMemoryAllocatable, nodeAllocatablePods)

		//Loop around different conditions like OutOfDisk, MemoryPressure etc to check if their status is good.
		for j := 0; j < 4; j++ {
			conditionStatus := nodeDetails.Status.Conditions[j].Status
			if conditionStatus != "False" {
				fmt.Printf("Skipping node %s as it is not healthy\n", node)
				flagHealthy = false
				break
			}
		}

		if flagHealthy == true {
			healthyNodes = append(healthyNodes, node)
			nodesCPU = append(nodesCPU, nodeCPUAllocatable)
			nodesMemory = append(nodesMemory, nodeMemoryAllocatable)
			nodeAllocatablePods = append(nodeAllocatablePods, allocatablePods)
		}
	}

	return healthyNodes, nodesCPU, nodesMemory, nodeAllocatablePods
}

func getNonTerminatedPodsForNode(clientset *kubernetes.Clientset, node string) ([]string, []string) {
	nonTerminatedPods := make([]string, 0, 5)
	namespaces := make([]string, 0, 5)

	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node + ",status.phase!=Pending,status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown")

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		panic(err.Error())
	}

	noOfPods := len(pods.Items)

	for i := 0; i < noOfPods; i++ {
		namespace := pods.Items[i].Namespace
		pod := pods.Items[i].Name
		nonTerminatedPods = append(nonTerminatedPods, pod)
		namespaces = append(namespaces, namespace)
	}

	return nonTerminatedPods, namespaces
}

func getPodCPUMemoryRequestsLimits(clientset *kubernetes.Clientset, pods []string, namespaces []string) (uint64, uint64, int64, int64) {

	var cpuRequestsMiliTotal uint64 = 0
	var cpuLimitsMiliTotal uint64 = 0
	var memoryRequestsBytesTotal int64 = 0
	var memoryLimitsBytesTotal int64 = 0

	for i, pod := range pods {

		podDetails, err := clientset.CoreV1().Pods(namespaces[i]).Get(pod, metav1.GetOptions{})
		//fmt.Printf("\nPod details : %v\n\n", podDetails)

		if errors.IsNotFound(err) {
			fmt.Printf("Pod %s in namespace %s not found\n", pod, namespaces[i])
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %s in namespace %s: %v\n",
				pod, namespaces[i], statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			//fmt.Printf("Pod %s in namespace %s and node %s is %s\n", pod, namespaces[i], podDetails.Spec.NodeName, podDetails.Status.Phase)
			noOfContainersPerPod := len(podDetails.Spec.Containers)
			for j := 0; j < noOfContainersPerPod; j++ {

				cpuLimits := podDetails.Spec.Containers[j].Resources.Limits["cpu"]
				cpuLimitsMili := convertCPUToMilis(cpuLimits.String())

				cpuRequests := podDetails.Spec.Containers[j].Resources.Requests["cpu"]
				cpuRequestsMili := convertCPUToMilis(cpuRequests.String())

				memoryLimitsBytes := podDetails.Spec.Containers[j].Resources.Limits.Memory().Value()
				memoryRequestsBytes := podDetails.Spec.Containers[j].Resources.Requests.Memory().Value()

				//fmt.Printf("\n%s in %s details : %v %v %v %v\n",pod, namespaces[i], cpuLimitsMili, cpuRequestsMili, memoryLimitsBytes, memoryRequestsBytes)

				cpuRequestsMiliTotal = cpuRequestsMiliTotal + cpuRequestsMili
				cpuLimitsMiliTotal = cpuLimitsMiliTotal + cpuLimitsMili
				memoryRequestsBytesTotal = memoryRequestsBytesTotal + memoryRequestsBytes
				memoryLimitsBytesTotal = memoryLimitsBytesTotal + memoryLimitsBytes
			}
		}
	}
	//fmt.Printf("\nTotal CPU and Memory Limits and Requests : %v %v %v %v \n", cpuLimitsMiliTotal, cpuRequestsMiliTotal, memoryLimitsBytesTotal, memoryRequestsBytesTotal)
	return cpuLimitsMiliTotal, cpuRequestsMiliTotal, memoryLimitsBytesTotal, memoryRequestsBytesTotal
}

func convertCPUToMilis(cpu string) uint64 {

	flag := true
	if strings.HasSuffix(cpu, "m") {
		cpu = strings.TrimSuffix(cpu, "m")
		flag = false
	}

	cpuMili, converr := strconv.Atoi(cpu)
	if converr == nil {
		if flag == true {
			cpuMili = cpuMili * 1000
		}
	} else {
		cpuMili = 0
		fmt.Printf("\nError converting string to int for %s\n", cpu)
	}
	return uint64(cpuMili)
}
