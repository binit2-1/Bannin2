const parseBoolean = (value: string | undefined, defaultValue = false): boolean => {
  if (!value) return defaultValue;
  return value === "1" || value.toLowerCase() === "true";
};

const parseInteger = (
  value: string | undefined,
  defaultValue: number,
  bounds?: { min?: number; max?: number },
): number => {
  const parsed = Number.parseInt(value ?? "", 10);
  const finite = Number.isFinite(parsed) ? parsed : defaultValue;

  if (!bounds) return finite;

  const min = bounds.min ?? Number.NEGATIVE_INFINITY;
  const max = bounds.max ?? Number.POSITIVE_INFINITY;
  return Math.min(max, Math.max(min, finite));
};

export const env = {
  nodeEnv: process.env.NODE_ENV ?? "development",
  port: parseInteger(process.env.PORT, 6000, { min: 1, max: 65535 }),
  debugLogs: parseBoolean(process.env.DEBUG_AI, true),
  daemonBaseUrl: process.env.DAEMON_BASE_URL ?? "http://localhost:4000",
  reportUrlTtlSeconds: parseInteger(process.env.REPORT_URL_TTL_SECONDS, 600, {
    min: 300,
    max: 600,
  }),
  awsRegion: process.env.AWS_REGION,
  awsBucket: process.env.AWS_S3_BUCKET,
  redisUrl: process.env.REDIS_URL ?? "redis://localhost:6379",
  openAiBaseUrl: process.env.OPENAI_BASE_URL,
  openAiApiKey: process.env.OPENAI_API_KEY,
  openAiModel: process.env.OPENAI_MODEL ?? "gpt-5.4",
  openAiFastModel: process.env.OPENAI_FAST_MODEL ?? process.env.OPENAI_MODEL ?? "gpt-5.4-mini",
  databaseUrl: process.env.DATABASE_URL ?? "",
};
