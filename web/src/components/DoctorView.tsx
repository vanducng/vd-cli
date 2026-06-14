import { useEffect, useState } from "react";
import type { DoctorReport } from "../types";
import { api } from "../api";
import { DriftBadge } from "./DriftBadge";

export function DoctorView() {
  const [rep, setRep] = useState<DoctorReport | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    api.doctor().then(setRep).catch((e) => setErr(String(e)));
  }, []);

  if (err) return <p className="error">{err}</p>;
  if (!rep) return <p>Loading…</p>;

  return (
    <table>
      <thead>
        <tr>
          <th>Skill</th>
          <th>Status</th>
          <th>Detail</th>
        </tr>
      </thead>
      <tbody>
        {rep.entries.map((e) => (
          <tr key={e.skill}>
            <td>{e.skill}</td>
            <td>
              <DriftBadge drift={e.status} />
            </td>
            <td className="mono">{e.detail}</td>
          </tr>
        ))}
        {rep.entries.length === 0 && (
          <tr>
            <td colSpan={3} className="muted">
              no locked skills
            </td>
          </tr>
        )}
      </tbody>
    </table>
  );
}
