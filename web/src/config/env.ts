import { z } from "zod";

const envSchema = z.object({
  // Empty string means same-origin (prod, served via go:embed); non-empty must be a valid http(s) URL (dev proxy).
  VITE_API_BASE_URL: z
    .string()
    .optional()
    .default("")
    .refine((v) => v === "" || /^https?:\/\//.test(v), {
      message: "VITE_API_BASE_URL must be empty (same-origin) or an http(s) URL",
    }),
});

export const env = envSchema.parse({
  VITE_API_BASE_URL: import.meta.env.VITE_API_BASE_URL,
});
