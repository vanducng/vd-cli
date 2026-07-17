import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { AgentBadge } from "@/features/obs/components/agent-badge";
import { TokenCell } from "@/features/obs/components/token-cell";
import type { SkillSummary } from "@/features/obs/schemas";

const NONE = "(none)";
const COLS = 11;

function Count({ value }: { value: number }) {
  if (value === 0) return <span className="text-muted-foreground">—</span>;
  return <span className="tabular-nums">{value.toLocaleString()}</span>;
}

// A nil error rate means the skill drove no tool call; show a dash, never 0.0%.
function ErrRate({ rate }: { rate: number | null }) {
  if (rate === null) return <span className="text-muted-foreground">—</span>;
  const pct = rate * 100;
  return <span className={pct >= 5 ? "tabular-nums text-err" : "tabular-nums"}>{pct.toFixed(1)}%</span>;
}

interface SkillsTableProps {
  skills: SkillSummary[];
  isLoading?: boolean;
  error?: Error | null;
}

/** Per-skill rollup mirroring `vd obs skills`: SKILL AGENTS INV SESS SOLO CALLS
 * ERRS ERR% CORR ABRT TOKENS, errors-desc with the (none) bucket last. */
export function SkillsTable({ skills, isLoading, error }: SkillsTableProps) {
  return (
    <div className="overflow-x-auto rounded-md border border-border">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>Skill</TableHead>
            <TableHead>Agents</TableHead>
            <TableHead className="text-right">Inv</TableHead>
            <TableHead className="text-right">Sess</TableHead>
            <TableHead className="text-right">Solo</TableHead>
            <TableHead className="text-right">Calls</TableHead>
            <TableHead className="text-right">Errs</TableHead>
            <TableHead className="text-right">Err%</TableHead>
            <TableHead className="text-right">Corr</TableHead>
            <TableHead className="text-right">Abrt</TableHead>
            <TableHead className="text-right">Tokens</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {error ? (
            <TableRow>
              <TableCell colSpan={COLS} className="h-24 text-center text-err">
                {error.message}
              </TableCell>
            </TableRow>
          ) : isLoading ? (
            Array.from({ length: 6 }).map((_, i) => (
              <TableRow key={i} className="hover:bg-transparent">
                {Array.from({ length: COLS }).map((_, j) => (
                  <TableCell key={j}>
                    <Skeleton className="h-4 w-full max-w-[80px]" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : skills.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={COLS} className="h-24 text-center text-muted-foreground">
                No skill activity in this range yet.
              </TableCell>
            </TableRow>
          ) : (
            skills.map((s) => (
              <TableRow key={s.name} className={s.name === NONE ? "text-muted-foreground" : undefined}>
                <TableCell className="font-mono">{s.name}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {s.agents.length === 0 ? (
                      <span className="text-muted-foreground">—</span>
                    ) : (
                      s.agents.map((a) => <AgentBadge key={a} agent={a} />)
                    )}
                  </div>
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.invocations} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.sessions} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.solosessions} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.toolcalls} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.toolerrors} />
                </TableCell>
                <TableCell className="text-right">
                  <ErrRate rate={s.errrate} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.corrections} />
                </TableCell>
                <TableCell className="text-right">
                  <Count value={s.aborts} />
                </TableCell>
                <TableCell className="text-right">
                  <TokenCell tokens={s.tokens} />
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
