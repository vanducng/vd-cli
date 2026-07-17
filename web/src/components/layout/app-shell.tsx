import type { ReactNode } from "react";

import { TopNav } from "@/components/layout/top-nav";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col">
      <TopNav />
      {/* min-w-0: a flex child defaults to min-width:auto and won't shrink below
          its content, so without this a wide table pushes the page body sideways
          instead of scrolling inside its own overflow-x-auto container. */}
      <main className="mx-auto w-full min-w-0 max-w-[1280px] flex-1 px-4 py-6 sm:px-8">{children}</main>
    </div>
  );
}
