package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gorilla/mux"
)

var kubeconfig *string

func main() {
	kubeconfig = flag.String("kubeconfig", "./config", "absolute path to the kubeconfig file")
	flag.Parse()

	r := mux.NewRouter()
	r.HandleFunc("/api/pod", createPod).Methods("POST")

	fmt.Println("Starting server on port 4242")
	http.ListenAndServe(":4242", r)
}

func createPod(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
	}

	var pod v1.Pod
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err = decode(body, nil, &pod)

	if err != nil {
		http.Error(w, "Error decoding JSON", http.StatusBadRequest)
		return
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		http.Error(w, "Error building kubeconfig", http.StatusInternalServerError)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		http.Error(w, "Error creating clientset", http.StatusInternalServerError)
		return
	}

	// Remove the scheduling gates
	pod.Spec.SchedulingGates = nil

	// Send a response that we received the pod
	json.NewEncoder(w).Encode(pod.Spec)

	// print the pod json to console
	podJson, err := json.Marshal(pod)
	if err != nil {
		fmt.Printf("Error marshalling pod: %v\n", err)
		return
	}
	fmt.Println(string(podJson))

	// Inside your goroutine
	go func() {
		_, err = clientset.CoreV1().Pods(pod.Namespace).Update(context.Background(), &pod, metav1.UpdateOptions{})
		if err != nil {
			fmt.Printf("Error updating pod: %v\n", err)
			return
		}

		fmt.Printf("Updated pod %s", pod.Name)
	}()
}
