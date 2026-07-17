import { createFileRoute } from "@tanstack/react-router";

import { TopBar } from "@/components/layout/top-bar";
import { DoctorView } from "@/features/doctor";

export const Route = createFileRoute("/doctor")({
  component: DoctorPage,
});

function DoctorPage() {
  return (
    <div>
      <TopBar title="Doctor" subtitle="Locked-skill drift report" />
      <DoctorView />
    </div>
  );
}
