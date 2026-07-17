import { env } from "@/config/env";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${env.VITE_API_BASE_URL}${path}`);
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

export const apiClient = { get };
