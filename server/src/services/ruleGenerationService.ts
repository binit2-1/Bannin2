import type { toolname } from "../../generated/prisma/enums.js";
import { runRuleWriterAgent } from "../agents/rule-writer.js";
import {
  restartToolWithApi,
  validateRulesWithToolApiDetailed,
  writeWithToolApi,
} from "./daemonToolsClient.js";
import { createLogger } from "../utils/logger.js";

const logger = createLogger("rule-generation.service");

export type RuleGenerationResult = {
  outputFile: string;
  validationOutput: string;
  rulesBytes: number;
};

export const runRuleGenerationService = async (
  selectedTool: toolname,
  projectSummary?: string,
): Promise<RuleGenerationResult> => {
  logger.info("start", {
    tool: selectedTool,
    hasSummary: Boolean(projectSummary),
  });

  const draft = await runRuleWriterAgent(selectedTool, projectSummary);
  const validation = await validateRulesWithToolApiDetailed(selectedTool, draft.rules);
  if (!validation.ok) {
    throw new Error(`Generated rules failed validation: ${validation.output}`);
  }

  const writeResult = await writeWithToolApi(draft.outputFile, draft.rules);
  if (writeResult !== "success") {
    throw new Error(`Failed to write rules to daemon path: ${draft.outputFile}`);
  }

  const restartResult = await restartToolWithApi(selectedTool);
  if (restartResult !== "success") {
    throw new Error(`Failed to restart ${selectedTool} after writing rules`);
  }

  logger.info("completed", {
    tool: selectedTool,
    outputFile: draft.outputFile,
    rulesBytes: draft.rules.length,
  });

  return {
    outputFile: draft.outputFile,
    validationOutput: validation.output,
    rulesBytes: draft.rules.length,
  };
};
