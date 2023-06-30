package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeconfig *string

func main() {
	kubeconfig = flag.String("kubeconfig", "./config", "absolute path to the kubeconfig file")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err)
		return
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)

	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if len(pod.Spec.SchedulingGates) > 0 {
				go func() {
					fmt.Println("Scheduling gates found on", pod.Name)
					// wait 10 seconds
					time.Sleep(20 * time.Second)

					// fetch the latest pod info
					latestPod, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
					if err != nil {
						fmt.Println(err)
						return
					}

					// remove the scheduling gates
					latestPod.Spec.SchedulingGates = nil

					// update the pod
					_, err = clientset.CoreV1().Pods(pod.Namespace).Update(context.Background(), latestPod, metav1.UpdateOptions{})
					if err != nil {
						fmt.Println(err)
						return
					}

					fmt.Println("Scheduling gates removed from", pod.Name)
				}()
			}
		},
	})

	stopper := make(chan struct{})
	defer close(stopper)

	fmt.Println("Starting informer")
	factory.Start(stopper)

	// wait until we get a stop signal
	<-stopper
}
