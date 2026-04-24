import { readFile } from "node:fs/promises";
import path from "node:path";
import type { toolname } from "../../generated/prisma/enums.js";

export type RuleToolConfig = {
  outputFile: string;
  validateCommand: string;
  applyCommand: string;
  rawToolBlock: string;
};

export type RuleWritingGuidance = {
  skillContext: string;
  toolConfig: RuleToolConfig;
};

const skillDir = path.resolve(process.cwd(), "skills", "rule-writing-guidelines");
const banninConfigPath = path.join(skillDir, "bannin.yaml");

const extractToolBlock = (rawConfig: string, selectedTool: toolname): string => {
  const pattern = new RegExp(`\\n  ${selectedTool}:\\n([\\s\\S]*?)(?=\\n  [a-z]+:\\n|$)`);
  const match = rawConfig.match(pattern);
  if (!match || !match[1]) {
    throw new Error(`Missing tool configuration for ${selectedTool}`);
  }

  return match[1];
};

const extractQuotedValue = (block: string, key: string): string => {
  const pattern = new RegExp(`${key}:\\s*"([^"]+)"`);
  const match = block.match(pattern);
  if (!match || !match[1]) {
    throw new Error(`Missing "${key}" in rule-writing config`);
  }

  return match[1];
};

export const loadRuleWritingGuidance = async (
  selectedTool: toolname,
): Promise<RuleWritingGuidance> => {
  const filesToLoad = [
    path.join(skillDir, "SKILL.md"),
    banninConfigPath,
    path.join(skillDir, `${selectedTool}.md`),
  ];
  const [skillOverview, rawConfig, toolGuide] = await Promise.all(
    filesToLoad.map((file) => readFile(file, "utf-8")),
  );

  const rawToolBlock = extractToolBlock(rawConfig, selectedTool);
  const toolConfig: RuleToolConfig = {
    outputFile: extractQuotedValue(rawToolBlock, "output_file"),
    validateCommand: extractQuotedValue(rawToolBlock, "validate_command"),
    applyCommand: extractQuotedValue(rawToolBlock, "apply_command"),
    rawToolBlock,
  };

  return {
    skillContext: `--- SKILL OVERVIEW ---\n${skillOverview}\n\n--- TOOL CONFIG ---\n${rawConfig}\n\n--- TOOL GUIDE (${selectedTool}) ---\n${toolGuide}`,
    toolConfig,
  };
};
