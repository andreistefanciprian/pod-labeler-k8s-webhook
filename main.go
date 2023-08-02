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

const (
	jsonContentType = `application/json`
)

var (
	deserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

func main() {
	// Parse CLI params
	parameters := parseFlags()

	// Create a new https server
	httpsMux := mux.NewRouter()
	httpsMux.HandleFunc("/mutate", HandleMutate)
	httpsAddr := ":" + strconv.Itoa(parameters.httpsPort)
	httpsServer := http.Server{
		Addr:    httpsAddr,
		Handler: httpsMux,
	}

	// Start the https server
	log.Printf("Starting https Server on port %s", httpsAddr)
	err := httpsServer.ListenAndServeTLS(parameters.certFile, parameters.keyFile)
	if err != nil {
		log.Fatal(err)
	}
}

// ServerParameters struct holds the parameters for the webhook server.
type ServerParameters struct {
	httpsPort int    // https server port
	certFile  string // path to the x509 certificate for https
	keyFile   string // path to the x509 private key matching `CertFile`
}

// patchOperation is a JSON patch operation, see https://jsonpatch.com/
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func parseFlags() ServerParameters {
	var parameters ServerParameters

	// Define and parse CLI params using the "flag" package.
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
	// Step 1: Request validation (Valid requests are POST with Content-Type: application/json)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %s\n", err.Error()), http.StatusInternalServerError)
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		http.Error(w, fmt.Sprintf("Invalid content type %s\n", contentType), http.StatusBadRequest)
		return
	}

	// Step 2: Parse the AdmissionReview request.
	var admissionReviewReq v1beta1.AdmissionReview
	if _, _, err := deserializer.Decode(body, nil, &admissionReviewReq); err != nil {
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

	//  Use for debug purposes when needed
	log.Printf("New Admission Review Request is being processed: User: %v \t PodName: %v \n %+v",
		admissionReviewReq.Request.UserInfo.Username,
		podName,
		string(body),
	)

	// Step 3: Construct the AdmissionReview response.
	// Construct the JSON patch operation for adding the "webhook" label
	var patches []patchOperation
	labels := pod.ObjectMeta.Labels
	labels["webhook"] = "auto-labeled"
	patchOp := patchOperation{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: labels,
	}
	patches = append(patches, patchOp)

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
