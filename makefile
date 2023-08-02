DOCKER_HUB_USERNAME := andreistefanciprian
IMAGE_NAME := pod-labeler-k8s-webhook
DOCKER_IMAGE_NAME := $(DOCKER_HUB_USERNAME)/$(IMAGE_NAME)

build:
	docker build -t $(DOCKER_IMAGE_NAME) . -f infra/Dockerfile
	docker image push $(DOCKER_IMAGE_NAME)

template-webhook-manifest:
	SHA_DIGEST="$$(curl -s "https://registry.hub.docker.com/v2/repositories/$(DOCKER_IMAGE_NAME)/tags" | jq -r '.results | sort_by(.last_updated) | last .digest')"; \
	sed -e 's@LATEST_DIGEST@'"$$SHA_DIGEST"'@g' < infra/deployment_template.yaml > infra/deployment.yaml

install:
	helm upgrade --namespace default --install pod-labeler infra/pod-labeler --create-namespace

uninstall:
	helm uninstall pod-labeler --namespace default

test:
	kubectl apply -f infra/test.yaml
	kubectl get pods,deployments -n foo --show-labels
	kubectl get ns foo --show-labels

test-clean:
	kubectl delete -f infra/test.yaml --ignore-not-found=true

clean: uninstall test-clean 

check:
	helm list -A
	kubectl get MutatingWebhookConfiguration pod-labeler --ignore-not-found=true
	kubectl get pods,secrets,certificates -n default