# monitoring

Observability stack for the serverless POC, as a single chart with two
dependencies plus this repo's glue:

| Piece | Source | Role |
|---|---|---|
| Prometheus + Grafana | `kube-prometheus-stack` dependency | Storage, dashboards, scraping |
| OTel Collector | `opentelemetry-collector` dependency | Receives OTLP metrics pushed by webapp + worker, re-exposes them on `:8889` |
| `templates/otel-collector-servicemonitor.yaml` | this chart | Prometheus ← collector `:8889` |
| `templates/operator-servicemonitor.yaml` | this chart | Prometheus ← operator `/metrics` (in `appNamespace`, default `default`) |
| `templates/grafana-dashboard-configmap.yaml` + `dashboards/serverless-poc.json` | this chart | Custom dashboard, auto-provisioned via the Grafana sidecar's `grafana_dashboard: "1"` label |

## Install

```bash
helm dependency update .
helm upgrade --install monitoring . --namespace monitoring --create-namespace
```

...or just `make monitoring` from the repo root. The release/namespace
names matter: the webapp and operator charts' default OTLP endpoint is
`http://monitoring-opentelemetry-collector.monitoring.svc.cluster.local:4318`.
Install under a different release name or namespace and those two values
(`otel.exporterEndpoint` in each chart) must be overridden to match.

## Design notes

- **Two metric paths on purpose.** The operator is a long-running process
  with a controller-runtime Prometheus endpoint already built in — a
  ServiceMonitor is the natural fit. The worker is a Job pod that lives
  for seconds, which no scrape interval can catch: it *must* push, so it
  (and the webapp, for symmetry) speaks OTLP to the collector and flushes
  on exit.
- **`resource_to_telemetry_conversion`** is enabled on the collector's
  prometheus exporter so `service.name` (webapp / welcome-email-worker)
  survives as a plain `service_name` label.
- **POC conveniences, not production settings**: Grafana login is
  admin/admin, Alertmanager is disabled, retention is 24h, and
  `serviceMonitorSelectorNilUsesHelmValues: false` makes Prometheus pick
  up every ServiceMonitor in the cluster.
