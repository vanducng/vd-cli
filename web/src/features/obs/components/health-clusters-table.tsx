import { Component, Fragment, useMemo, useState, type ErrorInfo, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ChevronDown, ChevronRight } from "lucide-react";

import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PaginationBar } from "@/features/shared/components/data-table";
import { CopyButton } from "@/features/obs/components/copy-button";
import { formatCount } from "@/features/obs/lib/format";
import type { ErrorCluster, Trend } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

const COLS = 7;
const PAGE_SIZE = 25;
const SKILL_CHIP_LIMIT = 2;
const EVIDENCE_CHIP_LIMIT = 12;
const SKELETON_ROWS = 10;
const SIGNATURE_TRUNCATE_THRESHOLD = 85;
const SIGNATURE_HEAD_LEN = 32;
const SIGNATURE_TAIL_LEN = 38;
// Mirrors internal/obs/signature.go's clusterKeyPrefixLen: the backend hard-cuts
// a normalized signature at this many runes with no ellipsis, mid-word, when
// building the cluster key. A signature at/over this length is never the true
// ending — display must always mark it as cut off.
const BACKEND_HARD_CUT_LEN = 140;

// Leading markdown/heading junk ("### ", "* ") occasionally leaks into a raw
// error signature; strip it for display only — the underlying signature stays
// untouched since it is the dedup/grouping key (frozen contract with the Go side).
function stripLeadingMarkup(s: string): string {
  return s.replace(/^[#*\s]+/, "");
}

// A hard character-level cut (ours or the backend's) can land mid-word
// ("denied Patt"), which reads as a rendering bug rather than an intentional
// truncation. Back up to the last word boundary, but only when the dropped
// fragment is a plain truncated word (letters only) — a code/regex-like
// fragment ("(^|\/)node_modu") is itself the discriminating detail two
// otherwise-identical signatures differ by, and must not be discarded.
function trimTrailingPartialWord(s: string): string {
  const lastSpace = s.lastIndexOf(" ");
  if (lastSpace <= 0) return s;
  const fragment = s.slice(lastSpace + 1);
  if (!/^[A-Za-z]+$/.test(fragment)) return s;
  return s.slice(0, lastSpace);
}

// The tail slice can start mid-sentence right after real punctuation (e.g.
// "...window." + "BLOCKED..." slices to ". BLOCKED...") — placed right after
// our own "…", a leading period/colon/etc. reads as doubled punctuation
// ("….") rather than a clean continuation. Drop any leading non-word run.
function trimLeadingPunctuation(s: string): string {
  const stripped = s.replace(/^[^\w]+/, "");
  return stripped === "" ? s : stripped;
}

// End-truncation alone hides the discriminating tail on signatures that share
// a long common prefix (e.g. "PreToolUse:Bash hook error: [python3 <str>]:
// NOTE: ..."), so this keeps both ends. Every truncated result — ours or the
// backend's 140-rune hard cut — ends with a single visible "…"; a signature
// that fits whole keeps its true, untouched ending.
function truncateSignatureMiddle(s: string, rawLen: number): string {
  // rawLen callers pass .length (UTF-16 units); backend cut is in runes — compare on code points
  const backendCut = [...s].length >= BACKEND_HARD_CUT_LEN || rawLen >= BACKEND_HARD_CUT_LEN;
  if (s.length <= SIGNATURE_TRUNCATE_THRESHOLD && !backendCut) return s;
  const head = s.slice(0, SIGNATURE_HEAD_LEN);
  const tail = trimTrailingPartialWord(trimLeadingPunctuation(s.slice(-SIGNATURE_TAIL_LEN)));
  return `${head}…${tail}…`;
}

// All trend chips render as one neutral, equal-weight style: count and trend
// reliability are independent cues, so no color/italic may imply a verdict.
// "low sample" (the API value) is displayed as "low baseline" — the small
// number is the *prior* window used for comparison, not this cluster's count,
// which reads self-contradictory next to a high-count row.
const TREND_LABEL: Record<Trend, string> = {
  up: "↑",
  down: "↓",
  flat: "→",
  "low sample": "low baseline",
  "": "—",
};

const TREND_TITLE: Record<Trend, string> = {
  up: "trending up vs the prior window",
  down: "trending down vs the prior window",
  flat: "flat vs the prior window",
  "low sample": "the prior window's baseline was too small for a reliable trend",
  "": "no trend data",
};

function TrendChip({ trend }: { trend: Trend }) {
  return (
    <span
      className="rounded-pill border border-border px-2 py-0.5 text-xs text-muted-foreground"
      title={TREND_TITLE[trend] ?? "no trend data"}
    >
      {TREND_LABEL[trend] ?? "—"}
    </span>
  );
}

interface RowErrorBoundaryState {
  hasError: boolean;
}

// A render error inside one row's expand panel (e.g. an unexpected field
// shape from a version-skewed bundle/binary pairing) must never take down the
// whole table with it. React error boundaries only exist as class components.
class RowErrorBoundary extends Component<{ children: ReactNode }, RowErrorBoundaryState> {
  state: RowErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): RowErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("HealthClustersTable: row detail failed to render", error, info);
  }

  render() {
    if (this.state.hasError) {
      return <p className="text-sm text-err">Couldn't render this row's details.</p>;
    }
    return this.props.children;
  }
}

function buildInvestigationPrompt(cluster: ErrorCluster): string {
  const first = cluster.evidence[0];
  const tools = cluster.affectedtools.join(", ") || "(none recorded)";
  const evidenceLine = first
    ? `Evidence: vd obs show ${first.sessionid} --json (turn ${first.turnindex}).`
    : "Evidence: no sample session recorded.";
  const skillLine = cluster.suggestedfocus ? ` Skill file: ${cluster.suggestedfocus}` : "";
  return `Investigate this recurring agent error cluster: signature "${cluster.signature}". ${cluster.count} occurrences, tools: ${tools}. ${evidenceLine}${skillLine}`;
}

function ClusterDetail({ cluster }: { cluster: ErrorCluster }) {
  // Belt-and-suspenders past the zod .default([]): a version-skewed binary
  // predating the variants field must degrade to "no variants", never a crash.
  const variants = cluster.variants ?? [];
  // The backend caps variants at its top 5 by count, so the visible sum can
  // legitimately fall short of the cluster's total — the remainder line
  // makes that gap explicit instead of letting the numbers look wrong.
  const variantsSum = variants.reduce((sum, v) => sum + (v.count ?? 0), 0);
  const otherVariantsCount = cluster.count - variantsSum;

  return (
    <div className="grid gap-4">
      {variants.length > 1 && (
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">
            Variants (top 5) — merged family, verify these are the same cause
          </div>
          <div className="grid gap-1.5 rounded-sm border border-border bg-panel-2 p-2">
            {variants.map((v, i) => (
              <div key={`${i}-${v.signature ?? ""}`} className="flex items-start gap-3">
                <span className="w-12 shrink-0 text-right font-mono text-xs tabular-nums text-muted-foreground">
                  {formatCount(v.count ?? 0)}
                </span>
                <span className="whitespace-pre-wrap break-words font-mono text-xs text-muted-foreground">
                  {stripLeadingMarkup(v.signature ?? "").trim() || "(empty error)"}
                </span>
              </div>
            ))}
            {otherVariantsCount > 0 && (
              <div className="flex items-start gap-3">
                <span className="w-12 shrink-0 text-right font-mono text-xs tabular-nums text-faint">
                  +{formatCount(otherVariantsCount)}
                </span>
                <span className="font-mono text-xs text-faint">errors in other variants</span>
              </div>
            )}
          </div>
        </div>
      )}

      <div>
        <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">Sample</div>
        <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-3 font-mono text-xs text-muted-foreground">
          {cluster.sample}
        </pre>
      </div>

      {cluster.suggestedfocus && (
        <div className="flex items-center justify-between gap-3 rounded-sm border border-primary/40 bg-primary/[0.06] px-3 py-2">
          <div className="min-w-0">
            <div className="text-xs font-semibold uppercase tracking-wide text-primary">Suggested focus</div>
            <div className="truncate font-mono text-xs text-muted-foreground">{cluster.suggestedfocus}</div>
          </div>
          <CopyButton value={cluster.suggestedfocus} label="Copy path" />
        </div>
      )}

      {cluster.cooccurringskills.length > 0 && (
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">Co-occurring skills</div>
          <div className="flex flex-wrap gap-2">
            {cluster.cooccurringskills.map((s) => (
              <div key={s.name} className="flex items-center gap-1.5 rounded-pill border border-border py-0.5 pl-2 pr-0.5">
                <span className="text-xs text-muted-foreground">{s.name}</span>
                <CopyButton value={s.path} label="path" className="h-6 px-1.5" />
              </div>
            ))}
          </div>
        </div>
      )}

      {cluster.evidence.length > 0 && (
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">
            Evidence ({cluster.evidence.length} turns · {cluster.sessions.length} sessions)
          </div>
          <div className="flex max-h-40 flex-wrap gap-2 overflow-auto">
            {cluster.evidence.slice(0, EVIDENCE_CHIP_LIMIT).map((e) => (
              <Link
                key={e.turnid}
                to="/obs/sessions/$id"
                params={{ id: e.sessionid }}
                className="rounded-pill border border-border px-2 py-1 font-mono text-xs text-info hover:underline"
              >
                {e.sessionid.slice(0, 8)}·t{e.turnindex}
              </Link>
            ))}
          </div>
          {cluster.evidence.length > EVIDENCE_CHIP_LIMIT && (
            <p className="mt-1.5 text-xs text-faint">
              +{cluster.evidence.length - EVIDENCE_CHIP_LIMIT} more evidence refs — use `vd obs health --json` for the
              full list
            </p>
          )}
        </div>
      )}

      <div>
        <CopyButton value={buildInvestigationPrompt(cluster)} label="Copy investigation prompt" />
      </div>
    </div>
  );
}

interface HealthClustersTableProps {
  clusters: ErrorCluster[];
  isLoading?: boolean;
  error?: Error | null;
}

/** The health view's centerpiece: signature, count, trend, tools, skills,
 * sessions — expand a row for the raw sample, evidence links, co-occurring
 * skills, and an investigation-prompt template. Count and trend reliability
 * are separate cues (a row can read count=377, trend="low sample"), so the
 * row and its count never dim for an unreliable trend — only the chip does.
 * Client-paged like a sibling built on the shared PaginationBar (clusters can
 * run into the thousands, but need a custom expand-row per row, which the
 * generic column-def DataTable doesn't support). */
export function HealthClustersTable({ clusters, isLoading, error }: HealthClustersTableProps) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [page, setPage] = useState(0);

  const pageClusters = useMemo(
    () => clusters.slice(page * PAGE_SIZE, page * PAGE_SIZE + PAGE_SIZE),
    [clusters, page],
  );

  function toggle(index: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  const showSkeleton = isLoading && clusters.length === 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="relative">
        <div className="max-h-[75vh] overflow-auto rounded-md border border-border bg-panel">
        <Table>
          <TableHeader className="sticky top-0 z-10 bg-panel">
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-8" />
              <TableHead>Signature</TableHead>
              <TableHead className="hidden text-right sm:table-cell">Count</TableHead>
              <TableHead className="hidden sm:table-cell">Trend</TableHead>
              <TableHead>Tools</TableHead>
              <TableHead>Skills</TableHead>
              <TableHead className="text-right">Sessions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {error ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={COLS} className="h-[420px] text-center text-err">
                  {error.message}
                </TableCell>
              </TableRow>
            ) : showSkeleton ? (
              Array.from({ length: SKELETON_ROWS }).map((_, i) => (
                <TableRow key={i} className="hover:bg-transparent">
                  {Array.from({ length: COLS }).map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full max-w-[160px]" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : clusters.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={COLS} className="h-[420px] text-center text-muted-foreground">
                  No tool errors in this window.
                </TableCell>
              </TableRow>
            ) : (
              pageClusters.map((cluster, i) => {
                const index = page * PAGE_SIZE + i;
                const isOpen = expanded.has(index);
                const cleanedSignature = stripLeadingMarkup(cluster.signature).trim();
                const displaySignature = truncateSignatureMiddle(cleanedSignature, cluster.signature.length);
                return (
                  <Fragment key={index}>
                    <TableRow
                      className={cn("cursor-pointer even:bg-panel-2/30 hover:bg-panel-2", isOpen && "bg-panel-2")}
                      onClick={() => toggle(index)}
                    >
                      <TableCell>
                        {isOpen ? (
                          <ChevronDown className="h-3.5 w-3.5 text-faint" />
                        ) : (
                          <ChevronRight className="h-3.5 w-3.5 text-faint" />
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap font-mono text-xs" title={cluster.signature}>
                        <div>
                          {cleanedSignature === "" ? (
                            <span className="italic text-faint">(empty error)</span>
                          ) : (
                            displaySignature
                          )}
                        </div>
                        {/* <sm: Count/Trend columns are hidden (scrolled off the
                            viewport otherwise), so ranking still needs to read
                            at a glance without discovering horizontal scroll. */}
                        <div className="mt-1 flex items-center gap-2 sm:hidden">
                          <span className="tabular-nums font-semibold text-foreground">{formatCount(cluster.count)}</span>
                          <TrendChip trend={cluster.trend} />
                        </div>
                      </TableCell>
                      <TableCell className="hidden text-right tabular-nums font-semibold sm:table-cell">
                        {formatCount(cluster.count)}
                      </TableCell>
                      <TableCell className="hidden sm:table-cell">
                        <TrendChip trend={cluster.trend} />
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate font-mono text-xs text-muted-foreground">
                        {cluster.affectedtools.join(", ") || "—"}
                      </TableCell>
                      <TableCell className="max-w-[300px]">
                        {cluster.cooccurringskills.length === 0 ? (
                          <span className="text-xs text-faint">—</span>
                        ) : (
                          <div className="flex flex-nowrap items-center gap-1 overflow-hidden">
                            {cluster.cooccurringskills.slice(0, SKILL_CHIP_LIMIT).map((s) => (
                              <span
                                key={s.name}
                                className="shrink-0 whitespace-nowrap rounded-pill border border-border px-1.5 py-0.5 text-xs text-muted-foreground"
                              >
                                {s.name}
                              </span>
                            ))}
                            {cluster.cooccurringskills.length > SKILL_CHIP_LIMIT && (
                              <span
                                className="shrink-0 text-xs text-faint"
                                title={cluster.cooccurringskills
                                  .slice(SKILL_CHIP_LIMIT)
                                  .map((s) => s.name)
                                  .join(", ")}
                              >
                                +{cluster.cooccurringskills.length - SKILL_CHIP_LIMIT}
                              </span>
                            )}
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">
                        {cluster.sessions.length}
                      </TableCell>
                    </TableRow>
                    {isOpen && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={COLS} className="bg-background/40 p-4">
                          <RowErrorBoundary>
                            <ClusterDetail cluster={cluster} />
                          </RowErrorBoundary>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })
            )}
          </TableBody>
        </Table>
        </div>
        {/* Static right-edge fade — signals "more columns, scroll right" rather
            than "content is cut off"; a scroll-position-aware toggle isn't worth
            the complexity here. */}
        <div className="pointer-events-none absolute inset-y-0 right-0 w-6 rounded-r-md bg-gradient-to-l from-panel to-transparent" />
      </div>

      {clusters.length > PAGE_SIZE && (
        <PaginationBar
          offset={page * PAGE_SIZE}
          pageSize={PAGE_SIZE}
          total={clusters.length}
          canPrev={page > 0}
          canNext={(page + 1) * PAGE_SIZE < clusters.length}
          onPrev={() => setPage((p) => Math.max(0, p - 1))}
          onNext={() => setPage((p) => p + 1)}
        />
      )}
    </div>
  );
}
