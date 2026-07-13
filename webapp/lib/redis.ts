import Redis from "ioredis";

// Module-level singleton: reused across requests/warm lambda-style invocations
// instead of opening a new connection per request.
declare global {
  // eslint-disable-next-line no-var
  var __redisClient: Redis | undefined;
}

export function getRedisClient(): Redis {
  if (!global.__redisClient) {
    const address = process.env.REDIS_ADDRESS;
    if (!address) {
      throw new Error("REDIS_ADDRESS environment variable is not set");
    }
    const [host, port] = address.split(":");
    global.__redisClient = new Redis({
      host,
      port: port ? Number(port) : 6379,
      maxRetriesPerRequest: 3,
      lazyConnect: false,
    });
  }
  return global.__redisClient;
}

export const REDIS_STREAM = process.env.REDIS_STREAM ?? "welcome-email-queue";
