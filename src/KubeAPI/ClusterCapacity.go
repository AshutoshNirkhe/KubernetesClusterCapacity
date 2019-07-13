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

var isVerbose bool

func main() {

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	cpuRquestsStr := flag.String("cpuRequests", "100m", "CPU Requests either in cores(1) or milicores(250m)")
	cpuLimitsStr := flag.String("cpuLimits", "200m", "CPU Limits either in cores(2) or milicores(500m)")
	memRequestsStr := flag.String("memRequests", "100mb", "Memory requests either in GB(1) or milicores(250mb)")
	memLimitsStr := flag.String("memLimits", "200mb", "Memory limits either in GB(2) or milicores(500mb)")
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

	if len(os.Args) == 1 {
		isVerbose = false
	} else if os.Args[1] == "verbose" {
		isVerbose = true
	} else {
		isVerbose = false
	}

	if isVerbose == true {
		fmt.Printf("\nVerbose set to : %v", isVerbose)
	}
	//fmt.Printf("Type of clientset is : %T", clientset)

	nodes, nodesCPU, nodesMemory, nodeAllocatablePods := getHealthyNodes(clientset)
	if isVerbose == true {
		fmt.Printf("\nHealthy nodes are : %v\n\n", nodes)
		fmt.Printf("\nHealthy nodes CPU are : %v\n\n", nodesCPU)
		fmt.Printf("\nHealthy nodes Memory are : %v\n\n", nodesMemory)
		fmt.Printf("\nHealthy nodes Max Allocatable pods : %v\n\n", nodeAllocatablePods)
	}

//	totalPossibleMinReplicas := 0
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

/*		possibleMinCPUReplicas := int((nodesCPU[index] - cpuLimitsMiliTotal) / cpuRequests)
		possibleMinMemoryReplicas := int((nodesMemory[index] - memoryLimitsBytesTotal) / memRequests)
		minReplicas := findMin(possibleMinCPUReplicas, possibleMinMemoryReplicas)
		//fmt.Printf("\nPossible replicas with CPU and Memory limits: %v %v", possibleMinCPUReplicas, possibleMinMemoryReplicas)

		//Make sure we don't cross e.g. 110 pod limit
		if minReplicas >= nodeAllocatablePods[index] {
			minReplicas = nodeAllocatablePods[index] - len(pods)
		}
		fmt.Printf("\nMin replicas : %v", minReplicas)
		totalPossibleMinReplicas = totalPossibleMinReplicas + minReplicas
*/
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

	fmt.Println("===============================================================================================================================")
	//fmt.Printf("\n\t Total possible Min-Max replicas for the pod with required input specs : %v-%v", totalPossibleMinReplicas, totalPossibleMaxReplicas)
	fmt.Printf("\n\t Total possible replicas for the pod with required input specs : %v", totalPossibleMaxReplicas)
	if totalPossibleMaxReplicas >= replicas {
		fmt.Printf("\n\t So you can go ahead with deployment of %v pod replicas in the Kubernetes cluster!!\n\n", replicas)
	} else {
		fmt.Printf("\n\t Unfortunately Kubernetes cluster can't scehdule %v replicas. Please try again by reducing the number of replicas or/and cpu/memory resource requests. Exiting!!\n\n", replicas)
		//os.Exit(1)
	}
	fmt.Println("===============================================================================================================================")
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
	if isVerbose == true {
		fmt.Printf("\nThere are total %d nodes in the cluster\n\n", noOfNodes)
	}

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
			if isVerbose == true {
				fmt.Printf("%s is not a worker node..skipping\n", node)
			}
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
		if isVerbose == true {
			fmt.Printf("%s - \n", node)
		}
		for j := 0; j < 4; j++ {
			nodeCondition := nodeDetails.Status.Conditions[j].Type
			conditionStatus := nodeDetails.Status.Conditions[j].Status
			if isVerbose == true {
				fmt.Printf("\t\t%s : %s \n", nodeCondition, conditionStatus)
			}
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

	//fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node + ",status.phase!=" + string(api.PodSucceeded) + ",status.phase!=" + string(api.PodFailed))
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node + ",status.phase!=Pending,status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown")
	//fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node + ",status.phase=Running")

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		panic(err.Error())
	}

	noOfPods := len(pods.Items)

	//if isVerbose == true {
	//fmt.Printf("No of pods : %d\n\n", noOfPods)
	//}

	for i := 0; i < noOfPods; i++ {
		/*clength := len(pods.Items[i].Spec.Containers)
		for j := 0; j < clength ; j++ {
			podtemp := pods.Items[i].Spec.Containers[j].Resources.Requests["memory"]
			fmt.Printf("\npod details - %v %v", podtemp.String(), clength)
		}*/
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
			if isVerbose == true {
				fmt.Printf("Pod %s in namespace %s and node %s is %s\n", pod, namespaces[i], podDetails.Spec.NodeName, podDetails.Status.Phase)
			}

			noOfContainersPerPod := len(podDetails.Spec.Containers)
			for j := 0; j < noOfContainersPerPod; j++ {

				cpuLimits := podDetails.Spec.Containers[j].Resources.Limits["cpu"]
				cpuLimitsMili := convertCPUToMilis(cpuLimits.String())

				cpuRequests := podDetails.Spec.Containers[j].Resources.Requests["cpu"]
				cpuRequestsMili := convertCPUToMilis(cpuRequests.String())

				/*memoryLimits := podDetails.Spec.Containers[j].Resources.Limits["memory"]
				                        	memoryLimitsBytes, byteserr := bytefmt.ToBytes(memoryLimits.String())
				                        	if byteserr != nil {
				                                	//fmt.Printf("\nError converting to Bytes\n")
				                                	memoryLimitsBytes = 0
				                        	}

					                        memoryRequests := podDetails.Spec.Containers[j].Resources.Requests["memory"]
				        	                memoryRequestsBytes, byteserr := bytefmt.ToBytes(memoryRequests.String())
				                	        if byteserr != nil {
				                        	        //fmt.Printf("\nError converting to Bytes\n")
				                                	memoryRequestsBytes = 0
				                        	}*/

				memoryLimitsBytes := podDetails.Spec.Containers[j].Resources.Limits.Memory().Value()
				memoryRequestsBytes := podDetails.Spec.Containers[j].Resources.Requests.Memory().Value()

				//fmt.Printf("\nRaw values : %v %v %v %v\n", cpuLimits.String(), cpuRequests.String(), memoryLimits.String(), memoryRequests.String())
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
