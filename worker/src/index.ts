import os from "os";
import Redis from "ioredis";
import { createRedisClient, parseFields, requireEnv } from "./redis";
import { sendWelcomeEmail } from "./mailer";
import {
  emailSendDuration,
  emailsProcessed,
  messagesClaimed,
  shutdownTelemetry,
} from "./telemetry";

/**
 * A message claimed by a consumer that has been idle longer than this is
 * assumed to belong to a dead worker pod (crashed or failed mid-send) and
 * is eligible for re-claiming. Healthy single-shot workers finish in
 * seconds, so 60s is comfortably past any legitimate in-flight send.
 */
const STALE_CLAIM_MS = 60_000;

type StreamEntry = [id: string, fields: string[]];

/**
 * Recover a message left pending by a dead consumer before asking for new
 * ones. Without this, a message delivered to a worker that then died would
 * stay pending forever: XREADGROUP ">" only ever returns never-delivered
 * entries.
 */
async function claimStale(
  redis: Redis,
  stream: string,
  group: string,
  consumer: string
): Promise<StreamEntry | null> {
  const res = (await redis.xautoclaim(
    stream,
    group,
    consumer,
    STALE_CLAIM_MS,
    "0",
    "COUNT",
    1
  )) as [string, StreamEntry[], ...unknown[]];

  const entries = res[1] ?? [];
  // Entries XDEL'd from the stream can surface with nil fields — skip those.
  const entry = entries.find(([, fields]) => Array.isArray(fields));
  return entry ?? null;
}

/**
 * Single-shot worker: claim one stale pending message (from a dead prior
 * worker) or read one new message from the stream's consumer group, send
 * the welcome email, ack it, and exit. Run once per Kubernetes Job by the
 * QueueWorker operator — no long-lived process here.
 */
async function main(): Promise<void> {
  const stream = requireEnv("REDIS_STREAM");
  const group = requireEnv("REDIS_CONSUMER_GROUP");
  // Kubernetes (via the container runtime) sets HOSTNAME to the pod name,
  // giving each Job a unique consumer name within the group for free.
  const consumer = process.env.HOSTNAME ?? os.hostname();

  const redis = createRedisClient();
  // Whether it is safe to remove this pod's consumer entry from the group
  // on exit. Never true while a message is still pending on this consumer:
  // XGROUP DELCONSUMER discards the consumer's pending entries, which would
  // silently lose the message.
  let consumerIsClean = false;

  try {
    let entry = await claimStale(redis, stream, group, consumer);
    if (entry) {
      messagesClaimed.add(1, { source: "stale" });
    }

    if (!entry) {
      const res = await redis.xreadgroup(
        "GROUP",
        group,
        consumer,
        "COUNT",
        "1",
        "BLOCK",
        "2000",
        "STREAMS",
        stream,
        ">"
      );

      if (!res) {
        console.log("No pending messages, exiting.");
        consumerIsClean = true;
        return;
      }

      // res shape: [[streamName, [[id, fields], ...]]]
      const [[, entries]] = res as [string, StreamEntry[]][];
      entry = entries[0];
      messagesClaimed.add(1, { source: "new" });
    }

    const [id, fields] = entry;
    const { name, email } = parseFields(fields);

    if (!name || !email) {
      console.error(`Message ${id} is missing name/email fields, acking to avoid poison-pill retries`);
      emailsProcessed.add(1, { outcome: "malformed" });
      await redis.xack(stream, group, id);
      consumerIsClean = true;
      return;
    }

    const sendStart = Date.now();
    try {
      await sendWelcomeEmail(name, email);
      emailSendDuration.record((Date.now() - sendStart) / 1000);
      emailsProcessed.add(1, { outcome: "sent" });
      await redis.xack(stream, group, id);
      consumerIsClean = true;
      console.log(`Sent welcome email for message ${id}`);
    } catch (err) {
      emailSendDuration.record((Date.now() - sendStart) / 1000);
      emailsProcessed.add(1, { outcome: "failed" });
      console.error(`Failed to send welcome email for message ${id}:`, err);
      process.exitCode = 1;
    }
  } finally {
    if (consumerIsClean) {
      // Best-effort hygiene: one consumer entry is created per pod, so
      // without this the group's consumer list grows forever.
      await redis
        .xgroup("DELCONSUMER", stream, group, consumer)
        .catch(() => undefined);
    }
    redis.disconnect();
    await shutdownTelemetry();
  }
}

main().catch((err) => {
  console.error("Worker failed:", err);
  process.exitCode = 1;
});
