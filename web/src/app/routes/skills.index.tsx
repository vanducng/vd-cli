import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { TopBar } from "@/components/layout/top-bar";
import { InventoryView } from "@/features/inventory";

export const Route = createFileRoute("/skills/")({
  component: SkillsIndexPage,
});

function SkillsIndexPage() {
  const navigate = useNavigate();
  return (
    <div>
      <TopBar title="Skills" subtitle="Managed and discovered assets" />
      <InventoryView onOpen={(name) => navigate({ to: "/skills/$name", params: { name } })} />
    </div>
  );
}
