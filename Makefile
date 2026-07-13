# Local development on kind (3-node cluster: 1 control-plane + 2 workers).
#
#   make up          # cluster + images + both charts, end to end
#   make down        # delete the kind cluster
#
# Individual steps are exposed as their own targets so any one of them can
# be re-run in isolation (e.g. `make load deploy` after a code change).

CLUSTER_NAME := serverless-poc
KIND_CONFIG  := deploy/kind/kind-config.yaml
KUBE_CONTEXT := kind-$(CLUSTER_NAME)

WEBAPP_IMAGE   := serverless-poc/webapp:latest
OPERATOR_IMAGE := serverless-poc/operator:latest
WORKER_IMAGE   := serverless-poc/welcome-email-worker:latest

KUBECTL := kubectl --context $(KUBE_CONTEXT)
HELM    := helm --kube-context $(KUBE_CONTEXT)

# Real SMTP credentials: `make smtp-secrets`, fill in the (gitignored)
# file, then `make up`/`deploy` — it is passed to helm when it exists.
SMTP_SECRETS   := deploy/helm/operator/values-secrets.yaml
HELM_SMTP_ARGS := $(if $(wildcard $(SMTP_SECRETS)),-f $(SMTP_SECRETS))

.PHONY: up down cluster images load smtp-secrets deploy \
        deploy-operator deploy-webapp monitoring grafana \
        port-forward status clean

up: cluster images load deploy monitoring status ## Everything, end to end

cluster: ## Create the 3-node kind cluster (no-op if it already exists)
	@kind get clusters 2>/dev/null | grep -qx $(CLUSTER_NAME) \
		|| kind create cluster --config $(KIND_CONFIG)

images: ## Build all three container images
	docker build -t $(WEBAPP_IMAGE) ./webapp
	docker build -t $(OPERATOR_IMAGE) ./operator
	docker build -t $(WORKER_IMAGE) ./worker

load: ## Load the images into every kind node
	kind load docker-image --name $(CLUSTER_NAME) \
		$(WEBAPP_IMAGE) $(OPERATOR_IMAGE) $(WORKER_IMAGE)

smtp-secrets: ## Bootstrap the (gitignored) real-SMTP credentials file
	@test -f $(SMTP_SECRETS) \
		&& echo "$(SMTP_SECRETS) already exists — edit it directly" \
		|| { cp $(SMTP_SECRETS).example $(SMTP_SECRETS); \
		     echo "Created $(SMTP_SECRETS) — fill in your SMTP host/user/pass,"; \
		     echo "then run: make deploy"; }

deploy: deploy-operator deploy-webapp ## Install/upgrade both Helm charts

deploy-operator: ## Operator chart (pulls in the bundled Redis subchart)
	$(HELM) dependency update deploy/helm/operator
	$(HELM) upgrade --install operator deploy/helm/operator $(HELM_SMTP_ARGS)

deploy-webapp:
	$(HELM) upgrade --install webapp deploy/helm/webapp

monitoring: ## Grafana + Prometheus + OTel collector (+ custom dashboard)
	$(HELM) dependency update deploy/helm/monitoring
	$(HELM) upgrade --install monitoring deploy/helm/monitoring \
		--namespace monitoring --create-namespace --timeout 10m

grafana: ## Grafana UI on http://localhost:3001 (admin / admin)
	@echo "Grafana: http://localhost:3001  (login: admin / admin)"
	$(KUBECTL) -n monitoring port-forward svc/monitoring-grafana 3001:80

port-forward: ## Webapp on http://localhost:3000 (Ctrl-C to stop)
	@echo "webapp: http://localhost:3000"
	$(KUBECTL) port-forward svc/webapp 3000:3000

status: ## Show nodes, releases, and workloads
	$(KUBECTL) get nodes
	$(HELM) list
	$(KUBECTL) get pods,jobs,queueworkers

down: ## Delete the kind cluster (and everything in it)
	kind delete cluster --name $(CLUSTER_NAME)

clean: down ## Delete the cluster and the locally built images
	-docker rmi $(WEBAPP_IMAGE) $(OPERATOR_IMAGE) $(WORKER_IMAGE)
