# webapp

Helm chart for the Next.js form + `/api/subscribe` route. Installs and
upgrades independently of the `queueworker-operator` chart.

## Install

```bash
helm install webapp .
```

## Cross-chart coupling

`values.yaml`'s `redis.stream` must match the corresponding
`queueWorkers[].redis.stream` entry in `deploy/helm/operator/values.yaml`
(default: `welcome-email-queue` for both). These are two independent
charts by design — see the top-level [`README.md`](../../../README.md) for
why, and the comments in this chart's `values.yaml`.

By default, `redis.address` (`operator-redis-master:6379`) assumes the
operator chart was installed with `helm install operator ...` (Bitnami's
standard master Service naming: `<release-name>-redis-master`). Override
`redis.address` if you used a different release name for the operator
chart, or if pointing at an external/managed Redis instance.
