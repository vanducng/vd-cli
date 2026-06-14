import { useEffect, useState } from "react";
import type { HookAsset } from "../types";
import { api } from "../api";

export function HooksView() {
  const [hooks, setHooks] = useState<HookAsset[] | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    api
      .hooks()
      .then((r) => setHooks(r.hooks))
      .catch((e) => setErr(String(e)));
  }, []);

  if (err) return <p className="error">{err}</p>;
  if (!hooks) return <p>Loading…</p>;

  return (
    <table>
      <thead>
        <tr>
          <th>Event</th>
          <th>Command</th>
          <th>vd</th>
        </tr>
      </thead>
      <tbody>
        {hooks.map((h, i) => (
          <tr key={i}>
            <td>{h.name}</td>
            <td className="mono">{h.description}</td>
            <td>{h.frontmatter?.managedByVd ? <span className="badge drift-none">vd</span> : ""}</td>
          </tr>
        ))}
        {hooks.length === 0 && (
          <tr>
            <td colSpan={3} className="muted">
              no hooks registered
            </td>
          </tr>
        )}
      </tbody>
    </table>
  );
}
