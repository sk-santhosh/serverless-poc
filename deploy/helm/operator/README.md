# queueworker-operator

Standalone, semantically-versioned Helm chart for the generic `QueueWorker`
operator. Nothing in this chart is specific to the welcome-email use case —
it declares the operator Deployment/RBAC, the `QueueWorker` CRD, an optional
bundled Redis (Bitnami subchart), and a list of `QueueWorker` CRs driven
entirely by `values.yaml`. It's written so it could later be added as a
`dependencies:` entry in another chart's `Chart.yaml`.

## Install

```bash
helm dependency update .
helm install operator . -f values-secrets.yaml   # see below re: values-secrets.yaml
```

## Adding a new queue

Append another entry to `queueWorkers` in `values.yaml` (or an overrides
file) and `helm upgrade`. No template or Go code changes are needed — see
the commented-out `some-other-queue` example in `values.yaml`.

## CRD lifecycle (important, expected Helm behavior)

The `QueueWorker` CRD lives under `crds/`. Helm installs anything under
`crds/` automatically **before** any template in this chart is rendered,
but per [Helm's documented CRD limitations](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/):

- `helm upgrade` **never** updates an existing CRD, even if `crds/queueworker-crd.yaml` changes between chart versions.
- `helm uninstall` **never** removes the CRD (and, by extension, never removes any `QueueWorker` CRs still using it).

This is standard Helm behavior, not a bug in this chart. If you change the
CRD schema, apply it manually:

```bash
kubectl apply -f crds/queueworker-crd.yaml
```

## SMTP credentials

`templates/smtp-secret.yaml` renders the `smtp-credentials` Secret from the
`smtp.*` values. **Never** commit real SMTP credentials into `values.yaml`.
Instead, create an untracked `values-secrets.yaml` (already covered by the
repo's root `.gitignore`) with real values:

```yaml
smtp:
  host: smtp.your-provider.com
  port: 587
  user: real-user
  pass: real-password
  fromEmail: welcome@yourdomain.com
```

and install/upgrade with `-f values-secrets.yaml`, or pass individual values
with `--set smtp.pass=...`. For local development, point `smtp.host` /
`smtp.port` at a `maildev` container instead (default in `values.yaml`) so
no real emails are ever sent.

## Cross-chart coupling

Each `queueWorkers[].redis.stream` value must match the corresponding
stream name the `deploy/helm/webapp` chart is configured to publish to
(`REDIS_STREAM` there). These are two independently-installable charts by
design — this is documented, not auto-synced. See the top-level
[`README.md`](../../../README.md) and `deploy/helm/webapp/values.yaml`.

## Bundled Redis

`redis.enabled: true` (default) pulls in the Bitnami `redis` chart as a
real Helm dependency (`oci://registry-1.docker.io/bitnamicharts`), deployed
in `standalone` architecture with auth disabled, suitable for local/POC use
only. Set `redis.enabled: false` and point every `queueWorkers[].redis.address`
at an external/managed Redis instance for anything beyond that.
