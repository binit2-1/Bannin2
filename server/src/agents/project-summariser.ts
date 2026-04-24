import { Agent, run, tool } from "@openai/agents";
import { z } from "zod";
import path from "node:path";
import { model } from "./client.js";
import {
  direnumWithToolApi,
  readWithToolApi,
  writeWithToolApi,
} from "../services/daemonToolsClient.js";
import { createLogger } from "../utils/logger.js";
import { RedisSession } from "./memory/redis.js";

const logger = createLogger("agent.project-summariser");

const normalizePath = (value: string): string => path.posix.resolve(value.replaceAll("\\", "/"));

const isInsideProjectRoot = (candidatePath: string, projectRoot: string): boolean => {
  const resolvedRoot = normalizePath(projectRoot);
  const resolvedCandidate = normalizePath(candidatePath);
  return resolvedCandidate === resolvedRoot || resolvedCandidate.startsWith(`${resolvedRoot}/`);
};

const trimForContext = (content: string, maxChars = 120000): string => {
  if (content.length <= maxChars) return content;
  return `${content.slice(0, maxChars)}\n\n[TRUNCATED ${content.length - maxChars} CHARS]`;
};

export const PROJECT_SUMMARY_REMOTE_PATH = "~/.bannin/project-summary.md";

const createProjectSummariserAgent = (projectRoot: string) => {
  const enumerateProject = tool({
    name: "enumerateProject",
    description:
      "Enumerate directory/file structure from daemon context to discover candidate files.",
    parameters: z.object({
      path: z.string().describe("Root path to start enumeration, typically project root."),
      level: z.number().int().min(1).max(8).describe("Depth level for project enumeration."),
    }),
    async execute({ level, path }) {
      const response = await direnumWithToolApi(level, path);
      return response.contents;
    },
  });

  const readProjectFile = tool({
    name: "readProjectFile",
    description: "Read a project file from daemon host.",
    parameters: z.object({
      filePath: z.string().describe("Absolute file path inside target project root."),
    }),
    async execute({ filePath }) {
      if (!isInsideProjectRoot(filePath, projectRoot)) {
        return `error: path is outside project root (${projectRoot})`;
      }

      const response = await readWithToolApi(filePath);
      return trimForContext(response.contents);
    },
  });

  return new Agent({
    name: "Project Summariser Agent",
    instructions: `
You are a senior application security engineer.
Your task is to analyze a source code project and produce a summary used by rule writers for Falco, Suricata, Wazuh, and Zeek.

Critical objective:
- Reduce noisy alerts by capturing true expected behavior.
- Never suppress real threats. Do not recommend broad blind spots.

You must:
1) Enumerate and inspect the codebase with tools.
2) Identify runtime behavior, data flows, external communications, and privileged operations.
3) Distinguish expected-but-noisy behavior from suspicious behavior.
4) Produce a strict, structured summary with evidence-backed details.

Required output format (markdown):
- Project Overview
- Runtime Components and Process Behavior
- Network Behavior Baseline
- Filesystem and Sensitive Path Baseline
- Authentication/Identity/Privilege Patterns
- Scheduled Jobs, Automation, and Batch Flows
- Likely False-Positive Sources (with precise guardrails)
- Must-Detect Threat Behaviors (do not suppress)
- Tool-Specific Notes:
  - Falco
  - Suricata
  - Wazuh
  - Zeek
- Rule-Writing Inputs (final checklist for downstream rule writer)

Strict rules:
- Every suppression candidate must include a scoped condition and a reason.
- Never suggest suppressing exploit primitives (credential theft, shell-spawn abuse, persistence, privilege escalation, C2 beaconing, data exfiltration) without hard context boundaries.
- If evidence is weak, say "unknown" instead of guessing.
`,
    model,
    tools: [enumerateProject, readProjectFile],
  });
};

export async function runProjectSummariserAgent(projectRoot: string): Promise<string> {
  logger.info("start", { projectRoot });
  const session = new RedisSession(`project-summariser:${projectRoot}`);
  const agent = createProjectSummariserAgent(projectRoot);

  const input = `Analyze project at root path: ${projectRoot}

Build a summary optimized for high-signal, low-noise detection rule writing.`;

  const result = await run(agent, input, { session });
  const finalSummary = String(result.finalOutput ?? "").trim();

  if (!finalSummary) {
    throw new Error("Project summariser returned an empty summary");
  }

  const writeStatus = await writeWithToolApi(PROJECT_SUMMARY_REMOTE_PATH, finalSummary);
  if (!writeStatus.ok) {
    throw new Error(
      `Failed to write summary to daemon path ${PROJECT_SUMMARY_REMOTE_PATH}: ${writeStatus.output}`,
    );
  }

  logger.debug("summary written", {
    path: PROJECT_SUMMARY_REMOTE_PATH,
  });
  logger.info("completed", { projectRoot });
  return finalSummary;
}
