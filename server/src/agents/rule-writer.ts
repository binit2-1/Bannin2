import { Agent, run, tool } from "@openai/agents";
import { z } from "zod";
import { model } from "./client.js";
import {
  readWithToolApi,
} from "../services/daemonToolsClient.js";
import type { toolname } from "../../generated/prisma/enums.js";
import { PROJECT_SUMMARY_REMOTE_PATH } from "./project-summariser.js";
import { createLogger } from "../utils/logger.js";
import { RedisSession } from "./memory/redis.js";
import { loadRuleWritingGuidance } from "../services/ruleWritingGuidance.js";

const logger = createLogger("agent.rule-writer");

const readRemoteFile = tool({
  name: "readFromFile",
  description: "Read the current file contents from the daemon host before drafting updates.",
  parameters: z.object({
    filePath: z
      .string()
      .describe("Absolute source path, including file name and extension."),
  }),
  async execute({ filePath }) {
    return readWithToolApi(filePath);
  },
});

const loadProjectSummaryContext = async (projectSummary?: string): Promise<string> => {
  if (typeof projectSummary === "string" && projectSummary.trim().length > 0) {
    logger.debug("using inline project summary");
    return projectSummary.trim();
  }

  try {
    const storedSummary = await readWithToolApi(PROJECT_SUMMARY_REMOTE_PATH);
    if (storedSummary.contents.trim().length > 0) {
      logger.debug("using stored project summary", {
        path: PROJECT_SUMMARY_REMOTE_PATH,
      });
      return storedSummary.contents.trim();
    }
  } catch {
    // Fall through to conservative fallback behavior.
  }

  return "Project summary unavailable. Use conservative default detections. Do not add broad suppressions without explicit environment evidence.";
};

export type RuleWriterDraft = {
  outputFile: string;
  rules: string;
};

const createRuleWriterAgent = (skillContext: string, outputFile: string, selectedTool: toolname) =>
  new Agent({
    name: "Rule Writer Agent",
    instructions: `
You are a senior detection engineer focused on reducing noisy alerts while preserving high-signal detections.

Follow this workflow exactly:
1) Read the provided project summary if available and extract workload profile, known noisy behaviors, and critical assets.
2) Use the skill context to follow tool-specific syntax, placement, validation, and tuning guidance.
3) Generate a production-ready rules file for the selected tool.
4) Read the current custom rules file first if it exists so you can preserve safe local context and avoid unnecessary churn.
5) Add rationale comments near major rule blocks so operators understand why each rule exists and how it suppresses noise.
6) Return only the final complete rules file contents for ${outputFile}.

Hard requirements:
- Keep rules conservative: minimize false positives.
- Avoid generic catch-all detections without context constraints.
- Keep syntax valid for the specific tool.
- If project summary is missing, avoid suppression-heavy tuning and keep rules narrowly scoped.
- If uncertain, read current file first with readFromFile and preserve local context.
- Do not wrap the final rules in markdown fences.
- Do not describe what you did outside of the rules file itself.

Skill context:
${skillContext}

Selected tool: ${selectedTool}
Canonical output path: ${outputFile}
    `,
    model,
    tools: [readRemoteFile],
  });

export async function runRuleWriterAgent(
  selectedTool: toolname,
  projectSummary?: string,
): Promise<RuleWriterDraft> {
  logger.info("start", { tool: selectedTool });

  const session = new RedisSession(`rule-writer:${selectedTool}`);

  const guidance = await loadRuleWritingGuidance(selectedTool);
  const resolvedProjectSummary = await loadProjectSummaryContext(projectSummary);
  const agent = createRuleWriterAgent(
    guidance.skillContext,
    guidance.toolConfig.outputFile,
    selectedTool,
  );

  const input = `Task: Create custom detection rules for "${selectedTool}".

Project Summary (authoritative context from backend):
${resolvedProjectSummary}

Read the existing custom rules file first if it exists at:
${guidance.toolConfig.outputFile}`;

  const result = await run(agent, input, { session, maxTurns: 20 });
  const rules = String(result.finalOutput ?? "").trim();
  if (!rules) {
    throw new Error("Rule writer returned an empty rules file");
  }

  logger.info("completed", { tool: selectedTool });
  return {
    outputFile: guidance.toolConfig.outputFile,
    rules,
  };
}
