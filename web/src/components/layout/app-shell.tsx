import type { ReactNode } from "react";

import { AppSidebar } from "@/components/layout/app-sidebar";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-screen grid-cols-[auto_1fr]">
      <AppSidebar />
      <main className="mx-auto w-full max-w-[1200px] px-8 py-6">{children}</main>
    </div>
  );
}
