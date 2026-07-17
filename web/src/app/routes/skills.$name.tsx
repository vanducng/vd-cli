import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { TopBar } from "@/components/layout/top-bar";
import { SkillDetailView } from "@/features/inventory";

export const Route = createFileRoute("/skills/$name")({
  component: SkillDetailPage,
});

function SkillDetailPage() {
  const { name } = Route.useParams();
  const navigate = useNavigate();
  return (
    <div>
      <TopBar title={name} subtitle="Skill detail" />
      <SkillDetailView name={name} onBack={() => navigate({ to: "/skills" })} />
    </div>
  );
}
