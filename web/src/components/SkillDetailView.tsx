import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import type { SkillDetail } from "../types";
import { api } from "../api";
import { DriftBadge } from "./DriftBadge";

interface Props {
  name: string;
  onBack: () => void;
}

export function SkillDetailView({ name, onBack }: Props) {
  const [d, setD] = useState<SkillDetail | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    setD(null);
    setErr("");
    api.skill(name).then(setD).catch((e) => setErr(String(e)));
  }, [name]);

  return (
    <div className="detail">
      <button className="back" onClick={onBack}>
        ← Back
      </button>
      {err && <p className="error">{err}</p>}
      {!err && !d && <p>Loading…</p>}
      {d && (
        <>
          <h2>
            {d.name} <DriftBadge drift={d.drift} />
          </h2>
          <p className="muted mono">{d.path}</p>
          {d.frontmatter && Object.keys(d.frontmatter).length > 0 && (
            <table className="fm">
              <tbody>
                {Object.entries(d.frontmatter).map(([k, v]) => (
                  <tr key={k}>
                    <th>{k}</th>
                    <td>{typeof v === "string" ? v : JSON.stringify(v)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <div className="md">
            <ReactMarkdown>{d.body}</ReactMarkdown>
          </div>
        </>
      )}
    </div>
  );
}
