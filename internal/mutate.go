package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func mutatePod(
	admissionReviewRequest admissionv1.AdmissionReview,
	deserializer runtime.Decoder,
) (*admissionv1.AdmissionResponse, error) {
	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("did not receive pod, got %s", admissionReviewRequest.Request.Resource.Resource)
		err := fmt.Errorf(msg)
		return nil, err
	}

	// Decode the pod from the AdmissionReview.
	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("error decoding raw pod: %v", err)
		err = fmt.Errorf("%s: %w", msg, err)
		return nil, err
	}

	// Marshal the pod back into JSON so we can send it to the
	podJSON, err := json.Marshal(pod)
	if err != nil {
		msg := fmt.Sprintf("Error Encoding Pod: %v", err)
		err = fmt.Errorf("%s: %w", msg, err)
		return nil, err
	}

	// This is a temperary hack to get the pod to the dockerhost
	// Also useful for local testing.
	response, err := http.Post("http://dockerhost:4242/api/pod", "application/json", bytes.NewBuffer(podJSON))
	if err != nil || response.StatusCode != http.StatusOK {
		status := "unknown"
		body := "no response"
		if response != nil {
			status = strconv.Itoa(response.StatusCode)
			defer response.Body.Close()

			bodyBytes, readErr := io.ReadAll(response.Body)
			if readErr != nil {
				body = "failed to read response body"
			} else {
				body = string(bodyBytes)
			}
		}

		msg := fmt.Sprintf("Error making request. HTTP status: %s, error: %v, response body: %s", status, err, body)
		err = fmt.Errorf("%s: %w", msg, err)
		return nil, err
	}

	defer response.Body.Close()

	// Create a response that will add a label to the pod if it does
	// not already have a label with the key of "hello". In this case
	// it does not matter what the value is, as long as the key exists.
	admissionResponse := &admissionv1.AdmissionResponse{}
	var patch string
	patchType := admissionv1.PatchTypeJSONPatch
	if len(pod.Spec.SchedulingGates) == 0 {
		patch = `[{"op":"add","path":"/spec/schedulingGates","value":[{"name":"mcaq.me/test-gate"}]}]`
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
	}

	return admissionResponse, nil
}
