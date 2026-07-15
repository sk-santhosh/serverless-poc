# Local development on kind (3-node cluster: 1 control-plane + 2 workers),
# or on minikube with the three apps sandboxed under gVisor (runsc).
#
#   make up                    # kind (default): cluster + images + charts
#   make up PLATFORM=minikube  # minikube + gvisor RuntimeClass on all 3 apps
#   make down                  # delete the kind cluster / stop minikube
#
# Individual steps are exposed as their own targets so any one of them can
# be re-run in isolation (e.g. `make load deploy` after a code change),
# and every target honors PLATFORM.
PLATFORM ?= kind

CLUSTER_NAME := serverless-poc
KIND_CONFIG  := deploy/kind/kind-config.yaml

WEBAPP_IMAGE   := serverless-poc/webapp:latest
OPERATOR_IMAGE := serverless-poc/operator:latest
WORKER_IMAGE   := serverless-poc/welcome-email-worker:latest

# minikube's gvisor addon default image is missing from its registry, so
# pin the working one explicitly (--images takes the path within the
# registry; --registries supplies the registry host).
GVISOR_ADDON_IMAGE    := minikube/gvisor:v0.0.4@sha256:0f389d92114b6342bcdb971fc8e89e9d60683d49aa5e31b89d744ec420196fd9
GVISOR_ADDON_REGISTRY := registry.k8s.io

ifeq ($(PLATFORM),minikube)
KUBE_CONTEXT := minikube
# The gvisor addon installs a RuntimeClass named "gvisor" (handler: runsc);
# these flags opt all three apps into it. The bundled Redis and the
# monitoring stack subcharts stay on the default runtime.
#
# service.type=NodePort because `kubectl port-forward` cannot reach gVisor
# pods: it dials the pod netns loopback, but a sandboxed pod's sockets live
# in runsc's userspace netstack, so the dial is refused. Traffic addressed
# to the pod IP (Services, NodePorts, probes) works — hence
# `minikube service webapp` instead of a port-forward below.
HELM_WEBAPP_RUNTIME_ARGS   := --set runtimeClassName=gvisor \
                              --set service.type=NodePort
HELM_OPERATOR_RUNTIME_ARGS := --set operator.runtimeClassName=gvisor \
                              --set workerRuntimeClassName=gvisor
else
KUBE_CONTEXT := kind-$(CLUSTER_NAME)
endif

KUBECTL := kubectl --context $(KUBE_CONTEXT)
HELM    := helm --kube-context $(KUBE_CONTEXT)

# Real SMTP credentials: `make smtp-secrets`, fill in the (gitignored)
# file, then `make up`/`deploy` — it is passed to helm when it exists.
SMTP_SECRETS   := deploy/helm/operator/values-secrets.yaml
HELM_SMTP_ARGS := $(if $(wildcard $(SMTP_SECRETS)),-f $(SMTP_SECRETS))

.PHONY: up down cluster gvisor images load smtp-secrets deploy \
        deploy-operator deploy-webapp monitoring grafana webapp \
        port-forward status clean

up: cluster images load deploy monitoring status ## Everything, end to end

cluster: ## Create/start the cluster for $(PLATFORM) (no-op if already up)
ifeq ($(PLATFORM),minikube)
	@minikube status >/dev/null 2>&1 \
		|| minikube start --container-runtime=containerd
	@$(MAKE) gvisor
else
	@kind get clusters 2>/dev/null | grep -qx $(CLUSTER_NAME) \
		|| kind create cluster --config $(KIND_CONFIG)
endif

gvisor: ## Enable minikube's gvisor addon if it isn't already
	@minikube addons list 2>/dev/null | grep gvisor | grep -q enabled \
		&& echo "gvisor addon already enabled" \
		|| minikube addons enable gvisor \
			--images=GvisorAddon=$(GVISOR_ADDON_IMAGE) \
			--registries=GvisorAddon=$(GVISOR_ADDON_REGISTRY)

images: ## Build all three container images
	docker build -t $(WEBAPP_IMAGE) ./webapp
	docker build -t $(OPERATOR_IMAGE) ./operator
	docker build -t $(WORKER_IMAGE) ./worker

load: ## Load the images into the cluster's nodes
ifeq ($(PLATFORM),minikube)
	minikube image load $(WEBAPP_IMAGE)
	minikube image load $(OPERATOR_IMAGE)
	minikube image load $(WORKER_IMAGE)
else
	kind load docker-image --name $(CLUSTER_NAME) \
		$(WEBAPP_IMAGE) $(OPERATOR_IMAGE) $(WORKER_IMAGE)
endif

smtp-secrets: ## Bootstrap the (gitignored) real-SMTP credentials file
	@test -f $(SMTP_SECRETS) \
		&& echo "$(SMTP_SECRETS) already exists — edit it directly" \
		|| { cp $(SMTP_SECRETS).example $(SMTP_SECRETS); \
		     echo "Created $(SMTP_SECRETS) — fill in your SMTP host/user/pass,"; \
		     echo "then run: make deploy"; }

deploy: deploy-operator deploy-webapp ## Install/upgrade both Helm charts

deploy-operator: ## Operator chart (pulls in the bundled Redis subchart)
	$(HELM) dependency update deploy/helm/operator
	$(HELM) upgrade --install operator deploy/helm/operator \
		$(HELM_SMTP_ARGS) $(HELM_OPERATOR_RUNTIME_ARGS)

deploy-webapp:
	$(HELM) upgrade --install webapp deploy/helm/webapp \
		$(HELM_WEBAPP_RUNTIME_ARGS)

monitoring: ## Grafana + Prometheus + OTel collector (+ custom dashboard)
	$(HELM) dependency update deploy/helm/monitoring
	$(HELM) upgrade --install monitoring deploy/helm/monitoring \
		--namespace monitoring --create-namespace --timeout 10m

grafana: ## Grafana UI on http://localhost:4001 (admin / admin)
	@echo "Grafana: http://localhost:4001  (login: admin / admin)"
	$(KUBECTL) -n monitoring port-forward svc/monitoring-grafana 4001:80

webapp: ## Webapp on http://localhost:4000 (Ctrl-C to stop)
ifeq ($(PLATFORM),minikube)
	# gVisor pods can't be kubectl-port-forwarded (see note at the top);
	# this tunnels to the NodePort instead and prints the URL to open.
	minikube service webapp --url
else
	@echo "webapp: http://localhost:4000"
	$(KUBECTL) port-forward svc/webapp 4000:3000
endif

# Not `port-forward: webapp grafana` — prerequisites run sequentially, and
# the webapp target's foreground port-forward never exits, so grafana would
# never start. Run both in one shell instead; Ctrl-C stops the pair.
port-forward: ## Forward ports to the webapp and grafana
ifeq ($(PLATFORM),minikube)
	@echo "Grafana: http://localhost:4001  (login: admin / admin)"
	@echo "webapp:  URL printed below by 'minikube service'"
	@$(KUBECTL) -n monitoring port-forward svc/monitoring-grafana 4001:80 & \
	minikube service webapp --url & \
	wait
else
	@echo "webapp:  http://localhost:4000"
	@echo "Grafana: http://localhost:4001  (login: admin / admin)"
	@$(KUBECTL) port-forward svc/webapp 4000:3000 & \
	$(KUBECTL) -n monitoring port-forward svc/monitoring-grafana 4001:80 & \
	wait
endif

status: ## Show nodes, releases, and workloads
	$(KUBECTL) get nodes
	$(HELM) list
	$(KUBECTL) get pods,jobs,queueworkers

down: ## Delete the kind cluster / stop minikube
ifeq ($(PLATFORM),minikube)
	# Stop, don't delete: the minikube profile may host other work.
	# `minikube delete` is left as an explicit manual step.
	minikube stop
else
	kind delete cluster --name $(CLUSTER_NAME)
endif

clean: down ## Delete the cluster and the locally built images
	-docker rmi $(WEBAPP_IMAGE) $(OPERATOR_IMAGE) $(WORKER_IMAGE)
