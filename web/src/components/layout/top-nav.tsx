import { Link, useRouterState } from "@tanstack/react-router";
import { Activity, BarChart3, Package, Stethoscope, Webhook } from "lucide-react";

import { cn } from "@/lib/utils";

interface NavItem {
  to: "/skills" | "/hooks" | "/doctor" | "/obs/sessions" | "/obs/usage";
  label: string;
  icon: typeof Package;
}

const NAV: NavItem[] = [
  { to: "/skills", label: "Skills", icon: Package },
  { to: "/hooks", label: "Hooks", icon: Webhook },
  { to: "/doctor", label: "Doctor", icon: Stethoscope },
  { to: "/obs/sessions", label: "Sessions", icon: Activity },
  { to: "/obs/usage", label: "Usage", icon: BarChart3 },
];

/** Sticky console chrome: brand, view tabs, live-local status chip. Replaces the
 * old sidebar — the winning design reads as a command surface, not a site. */
export function TopNav() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/90 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1280px] items-center gap-4 px-4 sm:px-8">
        <Link to="/" className="flex items-baseline gap-1.5 font-bold">
          <span className="rounded-sm bg-primary px-1.5 py-0.5 text-xs leading-none text-primary-foreground">vd</span>
          <span className="text-sm tracking-tight">console</span>
        </Link>

        <nav className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" aria-label="Primary">
          {NAV.map(({ to, label, icon: Icon }) => {
            const active = pathname === to || pathname.startsWith(`${to}/`);
            return (
              <Link
                key={to}
                to={to}
                aria-label={label}
                className={cn(
                  "flex shrink-0 items-center gap-1.5 rounded-pill px-2.5 py-1.5 text-sm text-muted-foreground hover:text-foreground sm:px-3",
                  active && "bg-panel-2 font-medium text-foreground",
                )}
              >
                <Icon className="h-4 w-4 sm:h-3.5 sm:w-3.5" />
                <span className="hidden sm:inline">{label}</span>
              </Link>
            );
          })}
        </nav>

        <span className="hidden items-center gap-1.5 rounded-pill border border-border px-2.5 py-1 text-xs text-muted-foreground sm:flex">
          <span className="h-1.5 w-1.5 rounded-full bg-ok" aria-hidden />
          local
        </span>
      </div>
    </header>
  );
}
