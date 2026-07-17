import { createFileRoute, Outlet } from "@tanstack/react-router";

// Layout for the observability section (Sessions + Usage). Has children
// (obs.index, obs.sessions.*, obs.usage), so it must render <Outlet/>, not a
// page body (fastreact gotcha 3).
export const Route = createFileRoute("/obs")({
  component: () => <Outlet />,
});
