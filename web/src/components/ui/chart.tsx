import * as React from "react";
import {
  Legend as RechartsLegend,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
} from "recharts";

import { cn } from "@/lib/utils";

export interface ChartConfig {
  [key: string]: {
    label: string;
    color?: string;
  };
}

interface ChartContextValue {
  config: ChartConfig;
}

const ChartContext = React.createContext<ChartContextValue | null>(null);

function useChart(): ChartContextValue {
  const ctx = React.useContext(ChartContext);
  if (!ctx) throw new Error("Chart components must be used within a ChartContainer");
  return ctx;
}

/** shadcn-style Recharts wrapper: injects per-series CSS vars from `config` so
 * series colors are declared once and consumed by both the chart and its
 * tooltip/legend, then hands the child chart a ResponsiveContainer. */
export function ChartContainer({
  config,
  className,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  config: ChartConfig;
  children: React.ReactElement;
}) {
  const id = React.useId();
  const style = Object.entries(config).reduce<Record<string, string>>((acc, [key, entry]) => {
    if (entry.color) acc[`--color-${key}`] = entry.color;
    return acc;
  }, {});

  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-chart={id}
        className={cn("h-full w-full [&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground", className)}
        style={style as React.CSSProperties}
        {...props}
      >
        <ResponsiveContainer>{children}</ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  );
}

export const ChartTooltip = RechartsTooltip;
export const ChartLegend = RechartsLegend;

interface TooltipPayloadItem {
  dataKey?: string | number;
  name?: string | number;
  value?: number;
  color?: string;
}

export function ChartTooltipContent({
  active,
  payload,
  label,
  formatter,
}: {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  label?: string;
  formatter?: (value: number) => string;
}) {
  const { config } = useChart();
  if (!active || !payload?.length) return null;

  return (
    <div className="rounded-md border border-border bg-popover px-3 py-2 text-xs shadow-md">
      {label && <div className="mb-1 font-medium text-foreground">{label}</div>}
      <div className="flex flex-col gap-1">
        {payload
          .filter((item) => (item.value ?? 0) > 0)
          .map((item) => {
            const key = String(item.dataKey ?? item.name ?? "");
            const entry = config[key];
            return (
              <div key={key} className="flex items-center gap-2 text-muted-foreground">
                <span
                  className="h-2 w-2 shrink-0 rounded-[2px]"
                  style={{ background: item.color ?? `var(--color-${key})` }}
                />
                <span className="flex-1">{entry?.label ?? key}</span>
                <span className="font-mono tabular-nums text-foreground">
                  {formatter ? formatter(item.value ?? 0) : item.value}
                </span>
              </div>
            );
          })}
      </div>
    </div>
  );
}

// Accepts an explicit config so the legend can render outside ChartContainer
// (e.g. above the chart, in the same Card) without needing its context.
export function ChartLegendContent({ config: configProp }: { config?: ChartConfig } = {}) {
  const ctx = React.useContext(ChartContext);
  const config = configProp ?? ctx?.config;
  if (!config) return null;
  return (
    <div className="mb-2 flex flex-wrap gap-4 text-xs text-muted-foreground">
      {Object.entries(config).map(([key, entry]) => (
        <span key={key} className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-[2px]" style={{ background: entry.color ?? `var(--color-${key})` }} />
          {entry.label}
        </span>
      ))}
    </div>
  );
}
