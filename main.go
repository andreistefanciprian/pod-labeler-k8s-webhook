package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"k8s.io/api/admission/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

func main() {
	// Parse CLI params
	parameters := parseFlags()

	// Create a new http server
	httpMux := mux.NewRouter()
	httpMux.HandleFunc("/healthz", HandleHealthz)
	httpAddr := ":" + strconv.Itoa(parameters.httpPort)
	httpServer := http.Server{
		Addr:    httpAddr,
		Handler: httpMux,
	}

	// Start the http server in a separate goroutine
	go func() {
		log.Printf("Starting http Server on port %s", httpAddr)
		err := httpServer.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Create a new https server
	httpsMux := mux.NewRouter()
	httpsMux.HandleFunc("/mutate", HandleMutate)
	httpsAddr := ":" + strconv.Itoa(parameters.httpsPort)
	httpsServer := http.Server{
		Addr:    httpsAddr,
		Handler: httpsMux,
	}

	// Start the https server in a separate goroutine
	go func() {
		log.Printf("Starting https Server on port %s", httpsAddr)

		err := httpsServer.ListenAndServeTLS(parameters.certFile, parameters.keyFile)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Use a channel to block the main goroutine and keep the program running
	select {}
}

// ServerParameters struct holds the parameters for the webhook server.
type ServerParameters struct {
	httpsPort int    // https server port
	httpPort  int    // http server port
	certFile  string // path to the x509 certificate for https
	keyFile   string // path to the x509 private key matching `CertFile`
}

// patchOperation struct represents a JSON patch operation used in mutating Kubernetes resources.
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func parseFlags() ServerParameters {
	var parameters ServerParameters

	// Define and parse CLI params using the "flag" package.
	flag.IntVar(&parameters.httpPort, "httpPort", 8080, " Http server port (healthcheck endpoint).")
	flag.IntVar(&parameters.httpsPort, "httpsPort", 443, " Https server port (webhook endpoint).")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	return parameters
}

// HandleHealthz is a liveness probe.
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	log.Printf("Health check at %v\n", r.URL.Path)
	w.WriteHeader(http.StatusOK)
}

// HandleMutate is the HTTP handler function for the /mutate endpoint.
func HandleMutate(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %s\n", err.Error()), http.StatusInternalServerError)
		return
	}

	// //  Use for debug purposes when needed
	// err = ioutil.WriteFile("/tmp/request", body, 0644)
	// if err != nil {
	// 	http.Error(w, fmt.Sprintf("Failed to write request body to file: %s\n", err.Error()), http.StatusInternalServerError)
	// 	return
	// }

	var admissionReviewReq v1beta1.AdmissionReview
	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		http.Error(w, fmt.Sprintf("Could not deserialize request: %s\n", err.Error()), http.StatusBadRequest)
		return
	} else if admissionReviewReq.Request == nil {
		http.Error(w, "Malformed admission review (request is nil)", http.StatusBadRequest)
		return
	}

	var pod apiv1.Pod
	err = json.Unmarshal(admissionReviewReq.Request.Object.Raw, &pod)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not unmarshal pod on admission request: %s\n", err.Error()), http.StatusInternalServerError)
		return
	}

	// Get the pod name
	var podName string
	if len(pod.GetName()) > 0 {
		podName = pod.GetName()
	} else {
		podName = pod.GetGenerateName()
	}

	log.Printf("New Admission Review Request is being processed: User: %v \t PodName: %v \n",
		admissionReviewReq.Request.UserInfo.Username,
		podName,
	)

	var patches []patchOperation
	labels := pod.ObjectMeta.Labels
	labels["app"] = "auto-labeled"
	patches = append(patches, patchOperation{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: labels,
	})

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not marshal JSON patch: %s\n", err.Error()), http.StatusInternalServerError)
		return
	}

	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     admissionReviewReq.Request.UID,
			Allowed: true,
		},
	}

	admissionReviewResponse.Response.Patch = patchBytes

	bytes, err := json.Marshal(&admissionReviewResponse)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not marshal JSON Admission Response: %s\n", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	log.Printf("Updated labels for Pod %v: %v \n",
		admissionReviewReq.Request.Name,
		labels,
	)
	w.Write(bytes)
}
