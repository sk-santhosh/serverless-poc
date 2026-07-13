import Redis from "ioredis";

export function createRedisClient(): Redis {
  const address = requireEnv("REDIS_ADDRESS");
  const [host, port] = address.split(":");
  return new Redis({
    host,
    port: port ? Number(port) : 6379,
    maxRetriesPerRequest: 3,
  });
}

export function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value;
}

/**
 * Redis stream entries arrive as a flat [key, value, key, value, ...] array.
 * This turns that into a plain object of string fields.
 */
export function parseFields(fields: string[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (let i = 0; i < fields.length; i += 2) {
    result[fields[i]] = fields[i + 1];
  }
  return result;
}
