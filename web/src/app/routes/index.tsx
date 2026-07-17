import { useEffect, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";

import { TopBar } from "@/components/layout/top-bar";
import { api } from "../../api";
import type { Inventory } from "../../types";
import { InventoryView } from "../../components/InventoryView";
import { HooksView } from "../../components/HooksView";
import { DoctorView } from "../../components/DoctorView";
import { SkillDetailView } from "../../components/SkillDetailView";

// Temporary tab-based landing page: the pre-fastreact views (Skills/Hooks/Doctor) are
// mounted here wholesale, plus placeholders for the not-yet-built Observability views.
// Phase 3 replaces this with real /skills, /hooks, /doctor routes; phase 4-5 add /obs/*.
const tabSchema = z.enum(["skills", "hooks", "doctor", "sessions", "usage"]).catch("skills");

export const Route = createFileRoute("/")({
  validateSearch: z.object({ tab: tabSchema.default("skills") }),
  component: IndexPage,
});

const TITLES: Record<z.infer<typeof tabSchema>, { title: string; subtitle?: string }> = {
  skills: { title: "Skills", subtitle: "Managed and discovered assets" },
  hooks: { title: "Hooks", subtitle: "Registered hook commands" },
  doctor: { title: "Doctor", subtitle: "Locked-skill drift report" },
  sessions: { title: "Sessions", subtitle: "Coming soon" },
  usage: { title: "Usage", subtitle: "Coming soon" },
};

function IndexPage() {
  const { tab } = Route.useSearch();
  const [inv, setInv] = useState<Inventory | null>(null);
  const [err, setErr] = useState("");
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    if (tab !== "skills") return;
    api.inventory().then(setInv).catch((e) => setErr(String(e)));
  }, [tab]);

  useEffect(() => {
    setSelected(null);
  }, [tab]);

  const { title, subtitle } = TITLES[tab];

  return (
    <div>
      <TopBar title={title} subtitle={subtitle} />
      {err && <p className="text-err">{err}</p>}
      {tab === "skills" &&
        (selected ? (
          <SkillDetailView name={selected} onBack={() => setSelected(null)} />
        ) : (
          <InventoryView inv={inv} onOpen={setSelected} />
        ))}
      {tab === "hooks" && <HooksView />}
      {tab === "doctor" && <DoctorView />}
      {(tab === "sessions" || tab === "usage") && (
        <p className="text-sm text-muted-foreground">This view ships in a later phase.</p>
      )}
    </div>
  );
}
