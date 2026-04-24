import { toolname } from "../../generated/prisma/enums.js";
import { env } from "../config/env.js";
import { createLogger } from "../utils/logger.js";

const logger = createLogger("daemon.tools");

type ReadToolResponse = {
  filepath: string;
  contents: string;
};

type DirenumToolResponse = {
  contents: string;
};

export type DaemonValidationResult = {
  ok: boolean;
  output: string;
  status: number;
};

export type DaemonActionResult = {
  ok: boolean;
  output: string;
  status: number;
};

const daemonBaseUrl = env.daemonBaseUrl;

const buildUrl = (pathname: string, query: Record<string, string>): string => {
  const url = new URL(pathname, daemonBaseUrl);
  for (const [key, value] of Object.entries(query)) {
    url.searchParams.set(key, value);
  }
  return url.toString();
};

const parseSuccessOrError = async (response: Response): Promise<"success" | "error"> => {
  const raw = (await response.text()).trim().toLowerCase();
  if (raw === "success") return "success";
  return "error";
};

const requestDaemon = async (pathname: string, options: RequestInit, query: Record<string, string> = {}) =>
  fetch(buildUrl(pathname, query), options);

const parseValidationResult = async (response: Response): Promise<DaemonValidationResult> => {
  const rawResult = (await response.text()).trim();
  return {
    ok: response.ok,
    output: rawResult.length > 0 ? rawResult : "validation completed with no output",
    status: response.status,
  };
};

const parseActionResult = async (response: Response): Promise<DaemonActionResult> => {
  const rawResult = (await response.text()).trim();
  const normalized = rawResult.toLowerCase();

  return {
    ok: response.ok && normalized === "success",
    output: rawResult.length > 0 ? rawResult : response.statusText || "no response body",
    status: response.status,
  };
};

export const readWithToolApi = async (path: string): Promise<ReadToolResponse> => {
  logger.debug("read request", { path });
  const response = await requestDaemon("/tools/read", {
    method: "GET",
  }, { path });

  if (!response.ok) {
    logger.error("read failed", { path, status: response.status });
    throw new Error(`Daemon read failed with status ${response.status}`);
  }

  const payload = (await response.json()) as Partial<ReadToolResponse>;
  if (typeof payload.filepath !== "string" || typeof payload.contents !== "string") {
    logger.error("read invalid response", { path });
    throw new Error("Daemon read response is invalid");
  }

  logger.debug("read success", {
    path: payload.filepath,
    bytes: payload.contents.length,
  });
  return {
    filepath: payload.filepath,
    contents: payload.contents,
  };
};

export const writeWithToolApi = async (path: string, contents: string): Promise<DaemonActionResult> => {
  logger.debug("write request", { path, bytes: contents.length });
  const response = await requestDaemon("/tools/write", {
    method: "POST",
    headers: {
      "content-type": "application/json",
    },
    body: JSON.stringify({ contents }),
  }, { path });

  const result = await parseActionResult(response);
  if (!result.ok) {
    logger.error("write failed", { path, status: result.status, output: result.output });
    return result;
  }

  logger.debug("write result", { path, result: result.output });
  return result;
};

export const editWithToolApi = async (
  oldContents: string,
  newContents: string,
  path?: string,
): Promise<"success" | "error"> => {
  logger.debug("edit request", {
    path: path ?? null,
    oldBytes: oldContents.length,
    newBytes: newContents.length,
  });
  const response = await requestDaemon(
    "/tools/edit",
    {
    method: "POST",
    headers: {
      "content-type": "application/json",
    },
    body: JSON.stringify({
      oldContents,
      newContents,
      ...(path ? { path } : {}),
    }),
    },
    path ? { path } : {},
  );

  if (!response.ok) {
    logger.error("edit failed", { status: response.status });
    return "error";
  }
  const result = await parseSuccessOrError(response);
  logger.debug("edit result", { result });
  return result;
};

export const restartToolWithApi = async (tool: toolname): Promise<DaemonActionResult> => {
  logger.debug("restart request", { tool });
  const response = await requestDaemon("/tools/restart", {
    method: "GET",
  }, { toolname: tool });

  const result = await parseActionResult(response);
  if (!result.ok) {
    logger.error("restart failed", { tool, status: result.status, output: result.output });
    return result;
  }

  logger.debug("restart result", { tool, result: result.output });
  return result;
};

export const direnumWithToolApi = async (level: number, path: string): Promise<DirenumToolResponse> => {
  logger.debug("direnum request", { level, path });
  const response = await requestDaemon("/tools/direnum", {
    method: "GET",
  }, { level: level.toString(), path });

  if (!response.ok) {
    logger.error("direnum failed", { level, path, status: response.status });
    throw new Error(`Daemon direnum failed with status ${response.status}`);
  }

  const payload = (await response.json()) as Partial<DirenumToolResponse>;
  if (typeof payload.contents !== "string") {
    logger.error("direnum invalid response", { level, path });
    throw new Error("Daemon direnum response is invalid");
  }

  logger.debug("direnum success", { level, path, bytes: payload.contents.length });
  return {
    contents: payload.contents,
  };
};

export const validateRulesWithToolApiDetailed = async (
  tool: toolname,
  rules: string,
): Promise<DaemonValidationResult> => {
  logger.debug("validate rules request", { tool, bytes: rules.length });

  const postResponse = await requestDaemon(
    "/tools/validate",
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify({ rules, toolname: tool }),
    },
    { toolname: tool },
  );

  let result = await parseValidationResult(postResponse);

  if (postResponse.status === 404 || postResponse.status === 405) {
    logger.warn("validate rules POST unsupported, falling back to GET", {
      tool,
      status: postResponse.status,
    });

    const fallbackResponse = await requestDaemon("/tools/validate", {
      method: "GET",
    }, { toolname: tool });

    result = await parseValidationResult(fallbackResponse);
  }

  if (!result.ok) {
    logger.error("validate rules failed", {
      tool,
      status: result.status,
      output: result.output,
    });
    return result;
  }

  logger.debug("validate rules result", {
    tool,
    status: result.status,
    output: result.output,
  });
  return result;
};

export const validateRulesWithToolApi = async (tool: toolname, rules: string): Promise<string> => {
  const result = await validateRulesWithToolApiDetailed(tool, rules);
  return result.output;
};
