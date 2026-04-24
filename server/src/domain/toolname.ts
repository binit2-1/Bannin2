export const toolname = {
  auditd: "auditd",
} as const;

export type toolname = (typeof toolname)[keyof typeof toolname];

export const TOOL_NAMES: toolname[] = Object.values(toolname);
