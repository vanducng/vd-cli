import { z } from "zod";

// Mirrors internal/obs/model/model.go exactly. Field names are flat lowercase
// json tags (costusd, cachehitrate, startedat), not camelCase. Do not "fix" the
// casing. See fastreact gotcha 2.

export const agentSchema = z.enum(["claude-code", "codex"]);
export type Agent = z.infer<typeof agentSchema>;

export const agentFilterSchema = z.union([z.literal("all"), agentSchema]);
export type AgentFilter = z.infer<typeof agentFilterSchema>;

export const tokenUsageSchema = z.object({
  input: z.number(),
  output: z.number(),
  cacheread: z.number(),
  cachewrite: z.number(),
  reasoningoutput: z.number(),
});
export type TokenUsage = z.infer<typeof tokenUsageSchema>;

// ReasoningOutput is already counted inside Output (see model.go), so it is
// deliberately excluded here too; summing it in would double the total.
export function totalTokens(t: TokenUsage): number {
  return t.input + t.output + t.cacheread + t.cachewrite;
}

const sessionBaseSchema = z.object({
  id: z.string(),
  agent: z.string(),
  title: z.string(),
  cwd: z.string(),
  project: z.string(),
  gitbranch: z.string(),
  gitsha: z.string(),
  model: z.string(),
  cliversion: z.string(),
  originator: z.string(),
  workflowid: z.string().optional(),
  parentid: z.string().optional(),
  startedat: z.coerce.date(),
  endedat: z.coerce.date(),
});

// costusd is nullable, not optional: an unpriced model always sends the field as
// null so the UI can render "?", never $0.00 (a zero reads as free).
export const sessionSummarySchema = sessionBaseSchema.extend({
  turncount: z.number(),
  tokens: tokenUsageSchema,
  costusd: z.number().nullable(),
  cachehitrate: z.number().nullable(),
});
export type SessionSummary = z.infer<typeof sessionSummarySchema>;

// {sessions, total, limit, offset} envelope, not a bare array.
export const sessionListSchema = z.object({
  sessions: z.array(sessionSummarySchema),
  total: z.number(),
  limit: z.number(),
  offset: z.number(),
});
export type SessionList = z.infer<typeof sessionListSchema>;

export const toolSpanSchema = z.object({
  id: z.string(),
  turnid: z.string(),
  name: z.string(),
  kind: z.string(),
  input: z.string(),
  output: z.string(),
  error: z.string(),
  durationms: z.number(),
  ok: z.boolean(),
  subagentsessionid: z.string().optional(),
  subagentname: z.string().optional(),
  rolluptokens: tokenUsageSchema.optional(),
  rollupcostusd: z.number().optional(),
});
export type ToolSpan = z.infer<typeof toolSpanSchema>;

export const hookExecSchema = z.object({
  turnid: z.string(),
  hookname: z.string(),
  event: z.string(),
  seq: z.number(),
  durationms: z.number(),
  exitcode: z.number(),
});
export type HookExec = z.infer<typeof hookExecSchema>;

export const skillSchema = z.object({
  turnid: z.string(),
  name: z.string(),
  seq: z.number(),
  args: z.string(),
});
export type Skill = z.infer<typeof skillSchema>;

export const turnSchema = z.object({
  id: z.string(),
  sessionid: z.string(),
  index: z.number(),
  model: z.string(),
  prompttext: z.string(),
  responsetext: z.string(),
  startedat: z.coerce.date(),
  durationms: z.number(),
  tokens: tokenUsageSchema,
  costusd: z.number().nullable(),
  toolspans: z.array(toolSpanSchema),
  hookexecs: z.array(hookExecSchema),
  skills: z.array(skillSchema),
});
export type Turn = z.infer<typeof turnSchema>;

export const sessionDetailSchema = sessionSummarySchema.extend({
  turns: z.array(turnSchema),
});
export type SessionDetail = z.infer<typeof sessionDetailSchema>;

export const usageRowSchema = z.object({
  date: z.string(),
  agent: z.string(),
  model: z.string(),
  tokens: tokenUsageSchema,
  costusd: z.number().nullable(),
});
export type UsageRow = z.infer<typeof usageRowSchema>;

export const usageReportSchema = z.object({
  rows: z.array(usageRowSchema),
  totals: tokenUsageSchema,
  totalcostusd: z.number().nullable(),
  unpricedmodels: z.array(z.string()),
});
export type UsageReport = z.infer<typeof usageReportSchema>;

// errrate is nullable, not optional: a skill that drove no tool call sends null so
// the UI renders "—", never 0.0% (a zero reads as "clean" when it means "no data").
export const skillSummarySchema = z.object({
  name: z.string(),
  agents: z.array(z.string()),
  invocations: z.number(),
  sessions: z.number(),
  solosessions: z.number(),
  toolcalls: z.number(),
  toolerrors: z.number(),
  errrate: z.number().nullable(),
  tokens: z.number(),
  corrections: z.number(),
  aborts: z.number(),
});
export type SkillSummary = z.infer<typeof skillSummarySchema>;

export const skillReportSchema = z.object({
  skills: z.array(skillSummarySchema),
});
export type SkillReport = z.infer<typeof skillReportSchema>;

// Frontend-only filter shapes. Field names mirror the HTTP query params 1:1
// (agent, project, q, since, limit, offset, sort) per Plan 1 phase-01.
export interface SessionFilter {
  agent?: Agent;
  project?: string;
  q?: string;
  since?: string;
  limit?: number;
  offset?: number;
  sort?: "newest" | "oldest";
}

export interface UsageFilter {
  group?: "daily" | "monthly";
  agent?: Agent;
  since?: string;
}

export interface SkillFilter {
  agent?: Agent;
  project?: string;
  since?: string;
}
