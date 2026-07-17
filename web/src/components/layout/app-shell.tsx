import type { ReactNode } from "react";

import { AppSidebar } from "@/components/layout/app-sidebar";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-screen grid-cols-[auto_1fr]">
      <AppSidebar />
      {/* min-w-0: a grid child defaults to min-width:auto and won't shrink below
          its content, so without this a wide table pushes the page body sideways
          instead of scrolling inside its own overflow-x-auto container. */}
      <main className="mx-auto w-full min-w-0 max-w-[1200px] px-4 py-6 sm:px-8">{children}</main>
    </div>
  );
}
