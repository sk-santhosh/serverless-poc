import { NextResponse } from "next/server";
import { randomUUID } from "crypto";
import { z } from "zod";
import { getRedisClient, REDIS_STREAM } from "@/lib/redis";
import { subscribeRequests } from "@/lib/telemetry";

const subscribeSchema = z.object({
  name: z.string().min(1, "Name is required"),
  email: z.string().email("A valid email is required"),
});

export async function POST(request: Request) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    subscribeRequests.add(1, { outcome: "invalid_json" });
    return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
  }

  const parsed = subscribeSchema.safeParse(body);
  if (!parsed.success) {
    subscribeRequests.add(1, { outcome: "invalid" });
    return NextResponse.json(
      { error: parsed.error.issues[0]?.message ?? "Invalid request" },
      { status: 400 }
    );
  }

  const { name, email } = parsed.data;
  const requestId = randomUUID();
  const submittedAt = new Date().toISOString();

  try {
    const redis = getRedisClient();
    await redis.xadd(
      REDIS_STREAM,
      "*",
      "name",
      name,
      "email",
      email,
      "requestId",
      requestId,
      "submittedAt",
      submittedAt
    );
  } catch (err) {
    // Log server-side only — never leak internals to the client.
    console.error("Failed to enqueue subscribe request", err);
    subscribeRequests.add(1, { outcome: "error" });
    return NextResponse.json(
      { error: "Failed to queue request. Please try again." },
      { status: 500 }
    );
  }

  subscribeRequests.add(1, { outcome: "accepted" });
  return NextResponse.json({ requestId }, { status: 202 });
}
