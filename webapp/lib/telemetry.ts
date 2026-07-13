import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import {
  MeterProvider,
  PeriodicExportingMetricReader,
} from "@opentelemetry/sdk-metrics";

/**
 * OpenTelemetry metrics, pushed as OTLP/HTTP to the collector named by the
 * standard OTEL_EXPORTER_OTLP_ENDPOINT env var (set in the webapp chart).
 * Push rather than a scrape endpoint keeps this identical to the worker's
 * setup, and the collector re-exposes everything to Prometheus.
 *
 * Stored on globalThis because Next.js may evaluate this module more than
 * once (dev hot-reload, route-level code splitting) and each MeterProvider
 * would otherwise start its own export loop.
 */
const globalKey = Symbol.for("serverless-poc.webapp.meterProvider");

function getMeterProvider(): MeterProvider {
  const g = globalThis as { [globalKey]?: MeterProvider };
  if (!g[globalKey]) {
    g[globalKey] = new MeterProvider({
      readers: [
        new PeriodicExportingMetricReader({
          exporter: new OTLPMetricExporter(),
          exportIntervalMillis: 10_000,
        }),
      ],
    });
  }
  return g[globalKey];
}

const meter = getMeterProvider().getMeter("webapp");

/**
 * One count per POST /api/subscribe, labeled with its outcome:
 * "accepted" | "invalid_json" | "invalid" | "error". The collector's
 * Prometheus exporter appends "_total".
 */
export const subscribeRequests = meter.createCounter(
  "webapp_subscribe_requests",
  { description: "Subscribe form submissions, by outcome." }
);
