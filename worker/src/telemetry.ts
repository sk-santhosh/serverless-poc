import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import {
  MeterProvider,
  PeriodicExportingMetricReader,
} from "@opentelemetry/sdk-metrics";

/**
 * OpenTelemetry metrics, pushed as OTLP/HTTP to the collector named by the
 * standard OTEL_EXPORTER_OTLP_ENDPOINT env var (injected into worker Jobs
 * by the operator chart's otel ConfigMap).
 *
 * Push is the only model that works here: this process lives for a few
 * seconds per Job, far shorter than any Prometheus scrape interval. The
 * export interval is deliberately long — shutdownTelemetry()'s final flush
 * on exit is the delivery mechanism.
 */
const provider = new MeterProvider({
  readers: [
    new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter(),
      exportIntervalMillis: 60_000,
    }),
  ],
});

const meter = provider.getMeter("welcome-email-worker");

/** Messages claimed for processing, labeled source="stale" (XAUTOCLAIM
 * from a dead consumer) or source="new" (XREADGROUP). */
export const messagesClaimed = meter.createCounter("worker_messages_claimed", {
  description: "Queue messages claimed by a worker, by source.",
});

/** Welcome emails, labeled outcome="sent" | "failed" | "malformed". */
export const emailsProcessed = meter.createCounter("worker_emails_processed", {
  description: "Welcome-email processing attempts, by outcome.",
});

export const emailSendDuration = meter.createHistogram(
  "worker_email_send_duration",
  {
    description: "Wall time of the SMTP send.",
    unit: "s",
  }
);

/**
 * Flush pending metrics before the process exits. Bounded so an
 * unreachable collector can never hold a completed Job pod hostage.
 */
export async function shutdownTelemetry(): Promise<void> {
  await Promise.race([
    provider.shutdown().catch((err) => {
      console.error("Failed to flush telemetry:", err);
    }),
    new Promise<void>((resolve) => setTimeout(resolve, 5_000).unref()),
  ]);
}
