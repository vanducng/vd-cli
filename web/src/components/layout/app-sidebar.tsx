import { useState } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import { Activity, BarChart3, ChevronsLeft, ChevronsRight, Package, Stethoscope, Webhook } from "lucide-react";

import { Brand } from "@/components/layout/brand";
import { cn } from "@/lib/utils";

interface NavItem {
  to: "/skills" | "/hooks" | "/doctor";
  label: string;
  icon: typeof Package;
}

const CONTROL_PLANE: NavItem[] = [
  { to: "/skills", label: "Skills", icon: Package },
  { to: "/hooks", label: "Hooks", icon: Webhook },
  { to: "/doctor", label: "Doctor", icon: Stethoscope },
];

// No /obs/* routes exist yet (phase-04/05); render as disabled entries rather than
// Links to a path the generated route tree doesn't know about.
const OBSERVABILITY: { label: string; icon: typeof Activity }[] = [
  { label: "Sessions", icon: Activity },
  { label: "Usage", icon: BarChart3 },
];

export function AppSidebar() {
  const [collapsed, setCollapsed] = useState(false);
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  return (
    <aside
      className={cn(
        "flex flex-col border-r border-border bg-panel transition-[width]",
        collapsed ? "w-14" : "w-52",
      )}
    >
      {!collapsed && <Brand />}
      <nav className="flex-1 px-2">
        <div className="mb-4">
          {!collapsed && <div className="mb-1 px-3 text-xs uppercase tracking-wide text-faint">Control plane</div>}
          {CONTROL_PLANE.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className={cn(
                "flex items-center gap-2 rounded-sm px-3 py-2 text-sm text-muted-foreground hover:text-foreground",
                pathname.startsWith(to) && "bg-panel-2 text-foreground",
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {!collapsed && <span>{label}</span>}
            </Link>
          ))}
        </div>
        <div className="mb-4">
          {!collapsed && <div className="mb-1 px-3 text-xs uppercase tracking-wide text-faint">Observability</div>}
          {OBSERVABILITY.map(({ label, icon: Icon }) => (
            <div
              key={label}
              className="flex items-center gap-2 rounded-sm px-3 py-2 text-sm text-faint opacity-60"
              title="Ships in a later phase"
            >
              <Icon className="h-4 w-4 shrink-0" />
              {!collapsed && <span>{label}</span>}
            </div>
          ))}
        </div>
      </nav>
      <button
        type="button"
        onClick={() => setCollapsed((c) => !c)}
        className="m-2 flex items-center justify-center rounded-sm p-2 text-muted-foreground hover:bg-panel-2 hover:text-foreground"
        aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
      >
        {collapsed ? <ChevronsRight className="h-4 w-4" /> : <ChevronsLeft className="h-4 w-4" />}
      </button>
    </aside>
  );
}
