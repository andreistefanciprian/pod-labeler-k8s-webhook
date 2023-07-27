# Pod Labeler Mutating Webhook

## Overview

This project implements a Kubernetes MutatingAdmissionWebhook, serving as an [admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) in the Kubernetes cluster. 
The webhook intercepts Pod creation requests and automatically adds an extra label to Pods (eg: ```webhook=auto-labeled```) if their target namespace has the label ```pod-labeler=enabled```.

Additionally, the webhook code can be easily modified to perform various other changes to Pod objects, such as altering their names, adding security parameters or injecting a sidecar.

## Prerequisites

Before getting started with the webhook, ensure that the following tools and resources are available:

- **cfssl**: Required for generating TLS certificates.
- **Docker**: The webhook runs as a container, so Docker is necessary.
- **Kubernetes Cluster**: You'll need a running Kubernetes cluster where the webhook will be deployed.
- **Go**: The webhook is written in Go.

## Generate Certificates

Before deploying the webhook, you need to generate TLS certificates using cfssl. Follow these steps:

1. Generate the CA certificate:
   ```
   cfssl print-defaults config > infra/config.json
   cfssl print-defaults csr > infra/csr.json
   cfssl gencert -initca infra/csr.json | cfssljson -bare infra/ca
   ```

2. Generate TLS certificates for the webhook:
   ```
   cfssl gencert \
   -ca=infra/ca.pem \
   -ca-key=infra/ca-key.pem \
   -config=infra/config.json \
   -hostname="pod-labeler,pod-labeler.default.svc.cluster.local,pod-labeler.default.svc,localhost,127.0.0.1" \
   -profile=server \
   infra/pod-labeler-csr.json | cfssljson -bare infra/pod-labeler
   ```

## Create Webhook k8s cert secret

After generating the TLS certificates, create Kubernetes secret with webhook tls certs:

1. Create a Kubernetes Secret to store the TLS certificates:
   ```
   kubectl create secret tls pod-labeler-tls \
   --key=infra/pod-labeler-key.pem \
   --cert=infra/pod-labeler.pem \
   --dry-run=client -o yaml >infra/secret.yaml
   ```

## Build and Run the Webhook

Build, Register, Deploy and Test the webhook using the provided tasks:

1. Build and push the Docker image to the container registry:
   ```
   task build
   ```

2. Deploy the webhook to the Kubernetes cluster:
   ```
   task deploy
   ```

3. Register webhook:
   ```
   task register
   ```

4. Test webhook:
   ```
   # check logs while creating test Pods and Deployments
   kubectl logs -l app=pod-labeler -f

   # create Pods and Deployments
   kubectl apply -f infra/test.yaml
   kubectl get pods,deployments --show-labels -n foo

   # cleanup test pods
   kubectl delete -f infra/test.yaml
   ```

5. Unregister and Remove the webhook:
   ```
   task unregister
   task undeploy
   ```

Feel free to adjust the tasks and configurations as needed to fit your specific environment.

## License

This project is licensed under the [MIT License](LICENSE). Feel free to use and modify it according to your requirements.