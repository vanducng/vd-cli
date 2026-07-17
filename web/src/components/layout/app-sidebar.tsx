import { useState } from "react";
import { Link, useSearch } from "@tanstack/react-router";
import { Activity, BarChart3, ChevronsLeft, ChevronsRight, Package, Stethoscope, Webhook } from "lucide-react";

import { Brand } from "@/components/layout/brand";
import { cn } from "@/lib/utils";

type Tab = "skills" | "hooks" | "doctor" | "sessions" | "usage";

interface NavItem {
  tab: Tab;
  label: string;
  icon: typeof Package;
}

const CONTROL_PLANE: NavItem[] = [
  { tab: "skills", label: "Skills", icon: Package },
  { tab: "hooks", label: "Hooks", icon: Webhook },
  { tab: "doctor", label: "Doctor", icon: Stethoscope },
];

const OBSERVABILITY: NavItem[] = [
  { tab: "sessions", label: "Sessions", icon: Activity },
  { tab: "usage", label: "Usage", icon: BarChart3 },
];

export function AppSidebar() {
  const [collapsed, setCollapsed] = useState(false);
  const { tab: activeTab } = useSearch({ from: "/" });

  return (
    <aside
      className={cn(
        "flex flex-col border-r border-border bg-panel transition-[width]",
        collapsed ? "w-14" : "w-52",
      )}
    >
      {!collapsed && <Brand />}
      <nav className="flex-1 px-2">
        <NavGroup label="Control plane" items={CONTROL_PLANE} activeTab={activeTab} collapsed={collapsed} />
        <NavGroup label="Observability" items={OBSERVABILITY} activeTab={activeTab} collapsed={collapsed} />
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

function NavGroup({
  label,
  items,
  activeTab,
  collapsed,
}: {
  label: string;
  items: NavItem[];
  activeTab: Tab;
  collapsed: boolean;
}) {
  return (
    <div className="mb-4">
      {!collapsed && (
        <div className="mb-1 px-3 text-xs uppercase tracking-wide text-faint">{label}</div>
      )}
      {items.map(({ tab, label: itemLabel, icon: Icon }) => (
        <Link
          key={tab}
          to="/"
          search={{ tab }}
          className={cn(
            "flex items-center gap-2 rounded-sm px-3 py-2 text-sm text-muted-foreground hover:text-foreground",
            activeTab === tab && "bg-panel-2 text-foreground",
          )}
        >
          <Icon className="h-4 w-4 shrink-0" />
          {!collapsed && <span>{itemLabel}</span>}
        </Link>
      ))}
    </div>
  );
}
