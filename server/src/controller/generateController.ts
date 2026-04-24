import type { Request, Response } from "express";
import { toolname } from "../../generated/prisma/enums.js";
import { runProjectSummariserAgent } from "../agents/project-summariser.js";
import { getSingleQueryValue } from "../http/requestParsers.js";
import { createLogger } from "../utils/logger.js";
import { runRuleGenerationService } from "../services/ruleGenerationService.js";

const logger = createLogger("generate.controller");

const isValidToolName = (value: unknown): value is toolname =>
  typeof value === "string" &&
  Object.values(toolname).includes(value as toolname);

export const generateRules = async (req: Request, res: Response) => {
  try {
    const { contents } = req.body;
    const selectedTool = getSingleQueryValue(req.query.toolname);
    logger.debug("generate rules request", { tool: selectedTool ?? null });

    if (!isValidToolName(selectedTool)) {
      return res.status(400).send("error");
    }

    if (contents !== undefined && typeof contents !== "string") {
      return res.status(400).send("error");
    }

    const projectSummary =
      typeof contents === "string" && contents.trim().length > 0 ? contents.trim() : undefined;

    const result = await runRuleGenerationService(selectedTool, projectSummary);
    logger.info("generate rules completed", {
      tool: selectedTool,
      hasSummary: Boolean(projectSummary),
      outputFile: result.outputFile,
      rulesBytes: result.rulesBytes,
    });

    return res.status(200).send("success");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error("generate rules failed", { error: message });
    return res.status(500).send(message);
  }
};

export const generateSummary = async (req: Request, res: Response) => {
  try {
    const rootFromQuery = getSingleQueryValue(req.query.path);
    const rootFromBodyPath =
      typeof req.body?.path === "string" ? req.body.path : undefined;
    const rootFromBodyProjectRoot =
      typeof req.body?.projectRoot === "string" ? req.body.projectRoot : undefined;

    const projectRoot = rootFromQuery ?? rootFromBodyProjectRoot ?? rootFromBodyPath;
    if (!projectRoot || projectRoot.trim().length === 0) {
      return res.status(400).send("error");
    }

    logger.debug("generate summary request", { projectRoot: projectRoot.trim() });
    await runProjectSummariserAgent(projectRoot.trim());
    logger.info("generate summary completed", { projectRoot: projectRoot.trim() });
    return res.status(200).send("success");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error("generate summary failed", { error: message });
    return res.status(500).send(message);
  }
};
