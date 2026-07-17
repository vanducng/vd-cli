import ReactMarkdown from "react-markdown";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DriftBadge } from "@/features/shared/components/drift-badge";
import { useSkillDetail } from "../queries";

interface SkillDetailViewProps {
  name: string;
  onBack: () => void;
}

export function SkillDetailView({ name, onBack }: SkillDetailViewProps) {
  const { data, isLoading, error } = useSkillDetail(name);

  return (
    <div>
      <Button variant="outline" size="sm" className="mb-4" onClick={onBack}>
        ← Back
      </Button>
      {error && <p className="text-sm text-err">{error.message}</p>}
      {!error && isLoading && <Skeleton className="h-64" />}
      {data && (
        <>
          <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold">
            {data.name} <DriftBadge drift={data.drift} />
          </h2>
          <p className="mb-4 font-mono text-xs text-muted-foreground">{data.path}</p>
          {data.frontmatter && Object.keys(data.frontmatter).length > 0 && (
            <table className="mb-4 w-full rounded-md border border-border bg-panel text-sm">
              <tbody>
                {Object.entries(data.frontmatter).map(([k, v]) => (
                  <tr key={k} className="border-b border-border/55 last:border-b-0">
                    <th className="w-1/4 py-1.5 pl-3 pr-3 text-left font-medium text-faint">{k}</th>
                    <td className="py-1.5 pr-3">{typeof v === "string" ? v : JSON.stringify(v)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <div className="rounded-md border border-border bg-panel p-4 text-sm leading-relaxed">
            <ReactMarkdown>{data.body}</ReactMarkdown>
          </div>
        </>
      )}
    </div>
  );
}
