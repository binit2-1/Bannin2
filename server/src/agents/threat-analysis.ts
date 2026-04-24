import { Agent, run } from "@openai/agents";
import model from "./client.js";
import type { toolname } from "../domain/toolname.js";
import { createLogger } from "../utils/logger.js";
import { RedisSession } from "./memory/redis.js";

const logger = createLogger("agent.threat-analysis");

export const PROJECT_SUMMARY_PATH = "~/.bannin/project-summary.md";

export type ThreatAnalysisInput = {
  eventId: string;
  sourceTool: toolname;
  timestamp: Date;
  priority: string;
  description: string;
  rawPayload: unknown;
  count: number;
  projectSummary: string;
  relatedEvents: Array<{
    timestamp: Date;
    priority: string;
    description: string;
    count: number;
  }>;
};

const createThreatAnalysisAgent = () =>
  new Agent({
    name: "Threat Analysis Agent",
    instructions: `
You are a senior SOC threat analyst.
Your job is to determine whether a detection is likely genuine malicious behavior, likely false positive, or inconclusive.
Write with a professional incident response tone suitable for direct rendering into a stakeholder-facing PDF report.

Primary safety rule:
- Never recommend suppressions that can hide real attacks.
- Prefer scoped tuning based on exact context rather than broad disables.

When making a decision:
1) Use event payload evidence first.
2) Use project summary if exists to understand expected behavior.
3) Use prior related event patterns for recurrence and context.
4) Explicitly call out missing evidence.

Return markdown with exactly these sections:
- Verdict
- Confidence (0-100)
- Key Evidence
- Genuine Threat Indicators
- False Positive Indicators
- Immediate Response Recommendations
- Safe Tuning Recommendations (noise reduction without blinding detection)
- Additional Data Needed

Hard constraints:
- If uncertain, return "Inconclusive" and list data required to decide.
- Any suppression recommendation must be narrow (host/user/process/path/time/tool scope).
- Do not suggest blanket ignores for credential access, privilege escalation, persistence, lateral movement, command-and-control, or exfiltration behavior.
`,
    model,
  });

export async function runThreatAnalysisAgent(input: ThreatAnalysisInput): Promise<string> {
  logger.info("start", {
    eventId: input.eventId,
    tool: input.sourceTool,
  });
  const session = new RedisSession(`threat-analysis:${input.eventId}`);
  const agent = createThreatAnalysisAgent();

  const prompt = `Analyze this detection event.

Event:
- id: ${input.eventId}
- sourceTool: ${input.sourceTool}
- timestamp: ${input.timestamp.toISOString()}
- priority: ${input.priority}
- description: ${input.description}
- repeatCount: ${input.count}

Raw Payload JSON:
${JSON.stringify(input.rawPayload, null, 2)}

Project Summary:
${input.projectSummary}

Recent Related Events:
${JSON.stringify(
  input.relatedEvents.map((event) => ({
    timestamp: event.timestamp.toISOString(),
    priority: event.priority,
    description: event.description,
    count: event.count,
  })),
  null,
  2,
)}`;

  const result = await run(agent, prompt, { session });
  logger.info("completed", { eventId: input.eventId });
  return String(result.finalOutput ?? "").trim();
}
