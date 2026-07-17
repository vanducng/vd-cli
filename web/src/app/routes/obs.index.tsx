import { createFileRoute, redirect } from "@tanstack/react-router";

// /obs has no page of its own; land on Sessions.
export const Route = createFileRoute("/obs/")({
  beforeLoad: () => {
    throw redirect({ to: "/obs/sessions" });
  },
});
