import type { Request, Response } from "express";
import { prisma } from "../exports/prisma.js";
import { TOOL_NAMES, type toolname } from "../domain/toolname.js";
import { enqueueThreatAnalysis } from "../services/threatAnalysisQueue.js";
import { getSignedThreatReportUrl } from "../services/awsReportStorage.js";
import {
  getSingleQueryValue,
  parseBooleanInput,
  parseDateInput,
  parsePositiveIntInput,
} from "../http/requestParsers.js";
import { createLogger } from "../utils/logger.js";

const logger = createLogger("event.controller");

const isValidToolName = (value: unknown): value is toolname =>
  typeof value === "string" &&
  TOOL_NAMES.includes(value as toolname);

export const createEvent = async (req: Request, res: Response) => {
  try {
    const { sourceTool, timestamp, priority, description, rawPayload } = req.body;
    logger.debug("create request", {
      sourceTool: typeof sourceTool === "string" ? sourceTool : null,
      hasTimestamp: Boolean(timestamp),
      hasPayload: rawPayload !== undefined,
    });

    if (!isValidToolName(sourceTool)) {
      return res.status(400).json({ error: "Invalid sourceTool" });
    }

    if (typeof description !== "string" || description.trim().length === 0) {
      return res.status(400).json({ error: "description is required" });
    }

    const parsedTimestamp = timestamp ? parseDateInput(timestamp) : new Date();
    if (!parsedTimestamp) {
      return res.status(400).json({ error: "Invalid timestamp" });
    }

    // A "same threat type" is treated as same source tool + same description.
    const existing = await prisma.event.findFirst({
      where: {
        sourceTool,
        description: description.trim(),
      },
      orderBy: {
        timestamp: "desc",
      },
    });

    if (existing) {
      const updated = await prisma.event.update({
        where: { id: existing.id },
        data: {
          timestamp: parsedTimestamp,
          priority: existing.priority,
          rawPayload,
          count: { increment: 1 },
        },
      });

      logger.info("updated existing event", { eventId: updated.id });

      return res.status(200).json({
        success: true,
        id: updated.id,
        updatedExisting: true,
      });
    }

    const created = await prisma.event.create({
      data: {
        sourceTool,
        timestamp: parsedTimestamp,
        priority,
        description: description.trim(),
        rawPayload,
        reportUrl: "",
      },
    });

    logger.info("created event", { eventId: created.id });

    return res.status(201).json({
      success: true,
      id: created.id,
      updatedExisting: false,
    });
  } catch (error) {
    logger.error("create failed", { error: String(error) });
    return res.status(500).json({ error: "Failed to create event", details: String(error) });
  }
};

export const getAllEvents = async (req: Request, res: Response) => {
  try {
    const start = getSingleQueryValue(req.query.start);
    const end = getSingleQueryValue(req.query.end);
    const rows = getSingleQueryValue(req.query.rows);
    const latestFirstRaw = getSingleQueryValue(req.query.lf);
    const oldestFirstRaw = getSingleQueryValue(req.query.of);

    const parsedStart = start ? parseDateInput(start) : null;
    const parsedEnd = end ? parseDateInput(end) : null;
    const parsedRows = parsePositiveIntInput(rows, { max: 500 });
    const oldestFirst = parseBooleanInput(oldestFirstRaw) ?? false;
    const latestFirst = oldestFirst ? false : (parseBooleanInput(latestFirstRaw) ?? true);
    logger.debug("list request", {
      start: start ?? null,
      end: end ?? null,
      rows: parsedRows ?? 100,
      oldestFirst,
      latestFirst,
    });

    if (start && !parsedStart) return res.status(400).json({ error: "Invalid start date" });
    if (end && !parsedEnd) return res.status(400).json({ error: "Invalid end date" });
    if (rows !== undefined && parsedRows === null) {
      return res.status(400).json({ error: "rows must be a positive integer" });
    }

    const timestampFilter: { gte?: Date; lte?: Date } = {};
    if (parsedStart) timestampFilter.gte = parsedStart;
    if (parsedEnd) timestampFilter.lte = parsedEnd;
    const whereClause =
      Object.keys(timestampFilter).length > 0 ? { timestamp: timestampFilter } : undefined;

    const events = await prisma.event.findMany({
      ...(whereClause ? { where: whereClause } : {}),
      orderBy: { timestamp: latestFirst ? "desc" : "asc" },
      take: parsedRows ?? 100,
      select: {
        id: true,
        sourceTool: true,
        timestamp: true,
        priority: true,
        description: true,
        reportUrl: true,
        count: true,
        askedAnalysis: true,
        finished: true,
      },
    });

    const signedEvents = await Promise.all(
      events.map(async (event: any) => ({
        ...event,
        reportUrl: event.reportUrl ? await getSignedThreatReportUrl(event.reportUrl) : "",
      })),
    );

    logger.debug("list response", { count: signedEvents.length });

    return res.status(200).json(signedEvents);
  } catch (error) {
    logger.error("list failed", { error: String(error) });
    return res.status(500).json({ error: "Failed to fetch events", details: String(error) });
  }
};

export const getEventById = async (req: Request, res: Response) => {
  try {
    const uuid = getSingleQueryValue(req.params.uuid);
    logger.debug("get by id request", { eventId: uuid ?? null });
    if (!uuid) return res.status(400).json({ error: "Invalid event id" });
    const event = await prisma.event.findUnique({ where: { id: uuid } });

    if (!event) {
      logger.debug("event not found", { eventId: uuid });
      return res.status(404).json({ error: "Event not found" });
    }

    const signedReportUrl = event.reportUrl
      ? await getSignedThreatReportUrl(event.reportUrl)
      : "";
    logger.debug("get by id response", { eventId: uuid, hasReport: Boolean(signedReportUrl) });
    return res.status(200).json({ ...event, reportUrl: signedReportUrl });
  } catch (error) {
    logger.error("get by id failed", {
      eventId: req.params.uuid,
      error: String(error),
    });
    return res.status(500).json({ error: "Failed to fetch event", details: String(error) });
  }
};

export const analyseEvent = async (req: Request, res: Response) => {
  try {
    const uuid = getSingleQueryValue(req.params.uuid);
    logger.debug("analyse request", { eventId: uuid ?? null });
    if (!uuid) return res.status(400).json({ error: "Invalid event id" });
    const event = await prisma.event.findUnique({ where: { id: uuid } });

    if (!event) {
      return res.status(404).json({ error: "Event not found" });
    }

    await prisma.event.update({
      where: { id: uuid },
      data: {
        askedAnalysis: true,
        finished: false,
        reportUrl: "",
      },
    });

    const queued = enqueueThreatAnalysis(uuid);
    logger.debug("analysis requested", { eventId: uuid, queued });

    return res.status(200).json({ success: true, queued });
  } catch (error) {
    logger.error("failed to enqueue analysis", {
      eventId: req.params.uuid,
      error: String(error),
    });
    return res.status(500).json({ error: "Failed to start analysis", details: String(error) });
  }
};

export const getEventStatus = async (req: Request, res: Response) => {
  try {
    const uuid = getSingleQueryValue(req.params.uuid);
    logger.debug("status request", { eventId: uuid ?? null });
    if (!uuid) return res.status(400).json({ error: "Invalid event id" });
    const event = await prisma.event.findUnique({
      where: { id: uuid },
      select: {
        askedAnalysis: true,
        finished: true,
        reportUrl: true,
      },
    });

    if (!event) {
      return res.status(404).json({ error: "Event not found" });
    }

    const signedReportUrl =
      event.finished && event.reportUrl
        ? await getSignedThreatReportUrl(event.reportUrl)
        : "";

    logger.debug("status response", {
      eventId: uuid,
      askedAnalysis: event.askedAnalysis,
      finished: event.finished,
      hasReport: Boolean(signedReportUrl),
    });

    return res.status(200).json({
      askedAnalysis: event.askedAnalysis,
      finished: event.finished,
      reportUrl: signedReportUrl,
    });
  } catch (error) {
    logger.error("status failed", {
      eventId: req.params.uuid,
      error: String(error),
    });
    return res.status(500).json({ error: "Failed to fetch status", details: String(error) });
  }
};
