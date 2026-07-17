import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ChartContainer,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { formatUsd } from "@/features/obs/lib/format";
import type { UsageRow } from "@/features/obs/schemas";

const PALETTE = [
  "hsl(var(--claude))",
  "hsl(var(--codex))",
  "hsl(var(--info))",
  "hsl(var(--ok))",
  "hsl(var(--primary))",
  "hsl(var(--err))",
];

interface UsageChartProps {
  rows: UsageRow[];
  isLoading?: boolean;
  error?: Error | null;
}

/** Cost-over-time, stacked by model, built from the same rows usage-table renders
 * beneath it. Unpriced rows (costusd null) contribute no bar height here; the
 * warning banner above the chart is what keeps that non-silent. */
export function UsageChart({ rows, isLoading, error }: UsageChartProps) {
  const { config, chartData } = useMemo(() => {
    // Recharts reads a dotted dataKey ("gpt-5.6-sol") as a nested path and a
    // dotted CSS var name is an invalid ident — sanitize keys, keep the real
    // model name as the config label
    const safe = (m: string) => m.replace(/[^a-zA-Z0-9_-]/g, "_");
    const models = [...new Set(rows.map((r) => r.model || "(unknown)"))].sort();
    const cfg: ChartConfig = {};
    models.forEach((m, i) => {
      cfg[safe(m)] = { label: m, color: PALETTE[i % PALETTE.length] };
    });

    const byDate = new Map<string, Record<string, number>>();
    for (const row of rows) {
      if (row.costusd === null) continue;
      const key = safe(row.model || "(unknown)");
      const bucket = byDate.get(row.date) ?? {};
      bucket[key] = (bucket[key] ?? 0) + row.costusd;
      byDate.set(row.date, bucket);
    }
    const data = [...byDate.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, values]) => ({ date, ...values }));

    return { config: cfg, chartData: data };
  }, [rows]);

  if (error) {
    return (
      <Card className="flex h-[220px] items-center justify-center text-err">
        {error.message}
      </Card>
    );
  }

  if (isLoading) {
    return <Skeleton className="h-[220px] w-full" />;
  }

  if (chartData.length === 0) {
    return (
      <Card className="flex h-[220px] items-center justify-center text-muted-foreground">
        No priced usage in this range yet.
      </Card>
    );
  }

  const models = Object.keys(config);

  return (
    <Card className="p-4">
      <ChartLegendContent config={config} />
      <ChartContainer config={config} className="h-[180px]">
        <BarChart data={chartData} margin={{ left: 0, right: 0, top: 4, bottom: 0 }}>
          <CartesianGrid vertical={false} stroke="hsl(var(--border))" />
          <XAxis
            dataKey="date"
            tickLine={false}
            axisLine={false}
            fontSize={11}
            tickFormatter={(d: string) => d.slice(5)}
          />
          <YAxis tickLine={false} axisLine={false} fontSize={11} tickFormatter={formatUsd} width={56} domain={[0, (max: number) => Math.ceil((max * 1.1) / 100) * 100]} />
          <ChartTooltip content={<ChartTooltipContent formatter={formatUsd} />} cursor={{ fill: "hsl(var(--panel-2))" }} />
          {/* model names ("gpt-5.6-sol") are invalid CSS idents, so var(--color-…)
              silently drops the fill — pass the palette color directly */}
          {models.map((m, i) => (
            <Bar
              key={m}
              dataKey={m}
              stackId="cost"
              fill={PALETTE[i % PALETTE.length]}
              radius={[2, 2, 0, 0]}
              isAnimationActive={false}
            />
          ))}
        </BarChart>
      </ChartContainer>
    </Card>
  );
}
