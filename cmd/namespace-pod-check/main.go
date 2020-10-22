package main

import (
	"context"
	"fmt"
	"os"
	"time"

	kh "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var podName = "kuberhealthy-namespace-checker-pod"

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()
}

func main() {
	// create context
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	// create kubernetes client
	kubernetesClient, err := kubeClient.Create("")
	if err != nil {
		log.Errorln("Error creating kubeClient with error" + err.Error())
	}

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	// fetches kube proxy to see if it is ready
	err = nodeCheck.WaitForKubeProxy(ctx, kubernetesClient, "kuberhealthy", "kube-system")
	if err != nil {
		log.Errorln("Error waiting for kube proxy to be ready and running on the node with error:" + err.Error())
	}

	//create namespace interface
	nsi := kubernetesClient.CoreV1().Namespaces()

	listOpts := v1.ListOptions{}

	//create namespace list
	namespaces, err := nsi.List(ctx, listOpts)

	log.Infoln("Found", len(namespaces.Items), "namespaces")

	//range over namespaces and test deployment and deletion of test pods
	for _, n := range namespaces.Items {
		log.Infoln("DEPLOYING POD IN NAMESPACE", n.Namespace)
		err := deployPod(ctx, n.Namespace, podName, kubernetesClient)
		if err != nil {
			log.Error(err)
		}
		err = deletePod(ctx, n.Namespace, podName, kubernetesClient)
		if err != nil {
			log.Error(err)
		}
	}
	log.Info("namespace-pod-checker was able to succesfully deploy and delete test pods in", len(namespaces.Items), "namespaces")
	kh.ReportSuccess()
}

func deployPod(ctx context.Context, namespace string, name string, client *kubernetes.Clientset) error {

	//pod defination
	pod := getPodObject(name)

	opts := metav1.CreateOptions{}

	//create the pod in kubernetes cluster using the clientset
	pod, err := client.CoreV1().Pods(namespace).Create(ctx, pod, opts)
	if err != nil {
		reportErr := fmt.Errorf("Error deploying pod " + name + " in namespace: " + namespace)
		return reportErr
	}
	log.Infoln("Pod", name, "created successfully in namespace:", namespace)
	return nil
}

//deletePod deletes pod in namespace
func deletePod(ctx context.Context, namespace string, name string, client *kubernetes.Clientset) error {

	delOpts := v1.DeleteOptions{}
	err := client.CoreV1().Pods(namespace).Delete(ctx, name, delOpts)
	if err != nil {
		reportErr := fmt.Errorf("Error deleting pod " + name + " in namespace " + namespace)
		return reportErr
	}
	log.Infoln("Pod", name, "successfully deleted in namespace", namespace)
	return nil
}

//getPodObject returns container to run for test
func getPodObject(name string) *core.Pod {
	return &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: core.PodSpec{
			Containers: []core.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: core.PullIfNotPresent,
					Command: []string{
						"sleep",
						"3600",
					},
				},
			},
		},
	}
}

// ReportFailureAndExit logs and reports an error to kuberhealthy and then exits the program.
// If a error occurs when reporting to kuberhealthy, the program fatals.
func ReportFailureAndExit(err error) {
	// log.Errorln(err)
	err2 := kh.ReportFailure([]string{err.Error()})
	if err2 != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Succesfully reported error to kuberhealthy")
	os.Exit(0)
}