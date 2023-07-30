DOCKER_HUB_USERNAME := andreistefanciprian
IMAGE_NAME := pod-labeler-k8s-webhook
DOCKER_IMAGE_NAME := $(DOCKER_HUB_USERNAME)/$(IMAGE_NAME)

build:
	docker build -t $(DOCKER_IMAGE_NAME) . -f infra/Dockerfile
	docker image push $(DOCKER_IMAGE_NAME)

template-webhook-manifest:
	SHA_DIGEST="$$(curl -s "https://registry.hub.docker.com/v2/repositories/$(DOCKER_IMAGE_NAME)/tags" | jq -r '.results | sort_by(.last_updated) | last .digest')"; \
	sed -e 's@LATEST_DIGEST@'"$$SHA_DIGEST"'@g' < infra/deployment_template.yaml > infra/deployment.yaml

deploy: template-webhook-manifest
	kubectl create secret tls pod-labeler-tls --key=infra/pod-labeler-key.pem --cert=infra/pod-labeler.pem -n default --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f infra/deployment.yaml -n default

undeploy: unregister
	kubectl delete secret pod-labeler-tls -n default --ignore-not-found=true
	kubectl delete -f infra/deployment.yaml --ignore-not-found=true -n default

template-webhook-config:
	CA_PEM_B64=$$(openssl base64 -A < infra/ca.pem); \
	sed -e 's@CA_PEM_B64@'"$$CA_PEM_B64"'@g' < infra/webhook_template.yaml > infra/webhook.yaml

register: template-webhook-config deploy
	kubectl apply -f infra/webhook.yaml

unregister:
	kubectl delete MutatingWebhookConfigurations pod-labeler --ignore-not-found=true

test:
	kubectl apply -f infra/test.yaml
	kubectl get pods,deployments -n foo --show-labels
	kubectl get ns foo --show-labels

test-clean:
	kubectl delete -f infra/test.yaml --ignore-not-found=true

clean: undeploy test-clean
	rm -f infra/deployment.yaml infra/webhook.yaml

check:
	kubectl get MutatingWebhookConfiguration pod-labeler --ignore-not-found=true
	kubectl get pods,secrets -n default