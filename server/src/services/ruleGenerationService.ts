import type { toolname } from "../domain/toolname.js";
import { runRuleWriterAgent } from "../agents/rule-writer.js";
import {
  restartToolWithApi,
  validateRulesWithToolApiDetailed,
  writeWithToolApi,
} from "./daemonToolsClient.js";
import { createLogger } from "../utils/logger.js";

const logger = createLogger("rule-generation.service");
const MAX_RULE_REWRITE_ATTEMPTS = 4;

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

  let lastValidationError = "";
  let lastRules = "";
  let finalOutputFile = "";

  for (let attempt = 1; attempt <= MAX_RULE_REWRITE_ATTEMPTS; attempt += 1) {
    logger.info("draft attempt", { tool: selectedTool, attempt });

    const draft = await runRuleWriterAgent(selectedTool, projectSummary, {
      attemptNumber: attempt,
      previousRules: lastRules,
      validationError: lastValidationError,
    });
    finalOutputFile = draft.outputFile;
    lastRules = draft.rules;

    const validation = await validateRulesWithToolApiDetailed(selectedTool, draft.rules);
    if (!validation.ok) {
      lastValidationError = validation.output;
      logger.warn("draft validation failed", {
        tool: selectedTool,
        attempt,
        output: validation.output,
      });
      continue;
    }

    const writeResult = await writeWithToolApi(draft.outputFile, draft.rules);
    if (!writeResult.ok) {
      if (attempt < MAX_RULE_REWRITE_ATTEMPTS && /validation failed/i.test(writeResult.output)) {
        lastValidationError = writeResult.output;
        logger.warn("write rejected by daemon validation", {
          tool: selectedTool,
          attempt,
          output: writeResult.output,
        });
        continue;
      }

      throw new Error(`Failed to write rules to daemon path ${draft.outputFile}: ${writeResult.output}`);
    }

    const restartResult = await restartToolWithApi(selectedTool);
    if (!restartResult.ok) {
      if (attempt < MAX_RULE_REWRITE_ATTEMPTS && /validation failed/i.test(restartResult.output)) {
        lastValidationError = restartResult.output;
        logger.warn("restart blocked by daemon validation", {
          tool: selectedTool,
          attempt,
          output: restartResult.output,
        });
        continue;
      }

      throw new Error(`Failed to restart ${selectedTool} after writing rules: ${restartResult.output}`);
    }

    logger.info("completed", {
      tool: selectedTool,
      outputFile: draft.outputFile,
      rulesBytes: draft.rules.length,
      attempts: attempt,
    });

    return {
      outputFile: draft.outputFile,
      validationOutput: validation.output,
      rulesBytes: draft.rules.length,
    };
  }

  throw new Error(
    `Generated rules failed validation after ${MAX_RULE_REWRITE_ATTEMPTS} attempts for ${selectedTool} (${finalOutputFile}): ${lastValidationError || "no validation output returned"}`,
  );
};
