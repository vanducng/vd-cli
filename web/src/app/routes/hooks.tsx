import { createFileRoute } from "@tanstack/react-router";

import { TopBar } from "@/components/layout/top-bar";
import { HooksView } from "@/features/hooks";

export const Route = createFileRoute("/hooks")({
  component: HooksPage,
});

function HooksPage() {
  return (
    <div>
      <TopBar title="Hooks" subtitle="Registered hook commands" />
      <HooksView />
    </div>
  );
}
