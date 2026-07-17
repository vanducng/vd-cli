import { createFileRoute, redirect } from "@tanstack/react-router";

// Skills/hooks/doctor now live at their own routes; root has no page of its own.
export const Route = createFileRoute("/")({
  beforeLoad: () => {
    throw redirect({ to: "/skills" });
  },
});
