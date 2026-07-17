import { createFileRoute, Outlet } from "@tanstack/react-router";

// Layout for /obs/sessions: the list lives at obs.sessions.index.tsx, the
// transcript view lands at obs.sessions.$id.tsx in phase 5. Structuring this as
// a layout now means phase 5 slots in without restructuring (fastreact gotcha 3).
export const Route = createFileRoute("/obs/sessions")({
  component: () => <Outlet />,
});
