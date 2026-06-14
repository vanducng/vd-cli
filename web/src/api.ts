import type { Inventory, SkillDetail, HookAsset, DoctorReport } from "./types";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      /* non-JSON error body */
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

export const api = {
  inventory: () => get<Inventory>("/api/inventory"),
  skill: (name: string) => get<SkillDetail>(`/api/skills/${encodeURIComponent(name)}`),
  hooks: () => get<{ hooks: HookAsset[] }>("/api/hooks"),
  doctor: () => get<DoctorReport>("/api/doctor"),
};
