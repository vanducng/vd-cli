import { createFileRoute, Outlet } from "@tanstack/react-router";

// Layout: has a "skills.$name" child (fastreact gotcha 3), must render <Outlet/>,
// not a page, or /skills/:name would render this instead of the detail view.
export const Route = createFileRoute("/skills")({
  component: () => <Outlet />,
});
