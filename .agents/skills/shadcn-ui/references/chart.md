### shadcn/ui Chart Component - Installation

Source: https://ui.shadcn.com/docs/components/chart

The chart component in shadcn/ui is built on Recharts, providing direct access to all Recharts capabilities with consistent theming.

```bash
npx shadcn@latest add chart
```

---

### shadcn/ui Chart Component - Basic Usage

Source: https://ui.shadcn.com/docs/components/chart

The ChartContainer wraps your Recharts component and accepts a config prop for theming. Requires `min-h-[value]` for responsiveness.

```tsx
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltipContent } from "@/components/ui/chart";

const chartConfig = {
  desktop: {
    label: "Desktop",
    color: "var(--chart-1)",
  },
  mobile: {
    label: "Mobile",
    color: "var(--chart-2)",
  },
} satisfies import("@/components/ui/chart").ChartConfig;

const chartData = [
  { month: "January", desktop: 186, mobile: 80 },
  { month: "February", desktop: 305, mobile: 200 },
  { month: "March", desktop: 237, mobile: 120 },
];

export function BarChartDemo() {
  return (
    <ChartContainer config={chartConfig} className="min-h-[200px] w-full">
      <BarChart data={chartData}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="month" tickLine={false} axisLine={false} />
        <Bar dataKey="desktop" fill="var(--color-desktop)" radius={4} />
        <Bar dataKey="mobile" fill="var(--color-mobile)" radius={4} />
        <ChartTooltip content={<ChartTooltipContent />} />
      </BarChart>
    </ChartContainer>
  );
}
```

---

### shadcn/ui Chart Component - ChartConfig with Custom Colors

Source: https://ui.shadcn.com/docs/components/chart

You can define custom colors directly in the configuration using hex values or CSS variables.

```tsx
const chartConfig = {
  desktop: {
    label: "Desktop",
    color: "#2563eb",
    theme: {
      light: "#2563eb",
      dark: "#60a5fa",
    },
  },
  mobile: {
    label: "Mobile",
    color: "var(--chart-2)",
  },
} satisfies import("@/components/ui/chart").ChartConfig;
```

---

### shadcn/ui Chart Component - CSS Variables

Source: https://ui.shadcn.com/docs/components/chart

Add chart color variables to your globals.css for consistent theming.

```css
:root {
  /* Chart colors */
  --chart-1: oklch(0.646 0.222 41.116);
  --chart-2: oklch(0.6 0.118 184.704);
  --chart-3: oklch(0.546 0.198 38.228);
  --chart-4: oklch(0.596 0.151 343.253);
  --chart-5: oklch(0.546 0.158 49.157);
}

.dark {
  --chart-1: oklch(0.488 0.243 264.376);
  --chart-2: oklch(0.696 0.17 162.48);
  --chart-3: oklch(0.698 0.141 24.311);
  --chart-4: oklch(0.676 0.172 171.196);
  --chart-5: oklch(0.578 0.192 302.85);
}
```

---

### shadcn/ui Chart Component - Line Chart Example

Source: https://ui.shadcn.com/docs/components/chart

Creating a line chart with shadcn/ui charts component.

```tsx
import { Line, LineChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltipContent } from "@/components/ui/chart";

const chartConfig = {
  price: {
    label: "Price",
    color: "var(--chart-1)",
  },
} satisfies import("@/components/ui/chart").ChartConfig;

const chartData = [
  { month: "January", price: 186 },
  { month: "February", price: 305 },
  { month: "March", price: 237 },
  { month: "April", price: 203 },
  { month: "May", price: 276 },
];

export function LineChartDemo() {
  return (
    <ChartContainer config={chartConfig} className="min-h-[200px]">
      <LineChart data={chartData}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="month" tickLine={false} axisLine={false} />
        <YAxis tickLine={false} axisLine={false} tickFormatter={value => `$${value}`} />
        <Line dataKey="price" stroke="var(--color-price)" strokeWidth={2} dot={false} />
        <ChartTooltip content={<ChartTooltipContent />} />
      </LineChart>
    </ChartContainer>
  );
}
```

---

### shadcn/ui Chart Component - Area Chart Example

Source: https://ui.shadcn.com/docs/components/chart

Creating an area chart with gradient fill and legend.

```tsx
import { Area, AreaChart, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltipContent,
} from "@/components/ui/chart";

const chartConfig = {
  desktop: { label: "Desktop", color: "var(--chart-1)" },
  mobile: { label: "Mobile", color: "var(--chart-2)" },
} satisfies import("@/components/ui/chart").ChartConfig;

export function AreaChartDemo() {
  return (
    <ChartContainer config={chartConfig} className="min-h-[200px]">
      <AreaChart data={chartData}>
        <XAxis dataKey="month" tickLine={false} axisLine={false} />
        <YAxis tickLine={false} axisLine={false} />
        <Area
          dataKey="desktop"
          fill="var(--color-desktop)"
          stroke="var(--color-desktop)"
          fillOpacity={0.3}
        />
        <Area
          dataKey="mobile"
          fill="var(--color-mobile)"
          stroke="var(--color-mobile)"
          fillOpacity={0.3}
        />
        <ChartTooltip content={<ChartTooltipContent />} />
        <ChartLegend content={<ChartLegendContent />} />
      </AreaChart>
    </ChartContainer>
  );
}
```

---

### shadcn/ui Chart Component - Pie Chart Example

Source: https://ui.shadcn.com/docs/components/chart

Creating a pie/donut chart with shadcn/ui.

```tsx
import { Pie, PieChart } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltipContent,
} from "@/components/ui/chart";

const chartConfig = {
  chrome: { label: "Chrome", color: "var(--chart-1)" },
  safari: { label: "Safari", color: "var(--chart-2)" },
  firefox: { label: "Firefox", color: "var(--chart-3)" },
} satisfies import("@/components/ui/chart").ChartConfig;

const pieData = [
  { browser: "Chrome", visitors: 275, fill: "var(--color-chrome)" },
  { browser: "Safari", visitors: 200, fill: "var(--color-safari)" },
  { browser: "Firefox", visitors: 187, fill: "var(--color-firefox)" },
];

export function PieChartDemo() {
  return (
    <ChartContainer config={chartConfig} className="min-h-[200px]">
      <PieChart>
        <Pie
          data={pieData}
          dataKey="visitors"
          nameKey="browser"
          cx="50%"
          cy="50%"
          outerRadius={80}
        />
        <ChartTooltip content={<ChartTooltipContent />} />
        <ChartLegend content={<ChartLegendContent />} />
      </PieChart>
    </ChartContainer>
  );
}
```

---

### shadcn/ui ChartTooltipContent Props

Source: https://ui.shadcn.com/docs/components/chart

The ChartTooltipContent component accepts these props for customizing tooltip behavior.

| Prop            | Type                        | Default | Description           |
| --------------- | --------------------------- | ------- | --------------------- |
| `labelKey`      | string                      | "label" | Key for tooltip label |
| `nameKey`       | string                      | "name"  | Key for tooltip name  |
| `indicator`     | "dot" \| "line" \| "dashed" | "dot"   | Indicator style       |
| `hideLabel`     | boolean                     | false   | Hide label            |
| `hideIndicator` | boolean                     | false   | Hide indicator        |

---

### shadcn/ui Chart Component - Accessibility

Source: https://ui.shadcn.com/docs/components/chart

Enable keyboard navigation and screen reader support by adding the accessibilityLayer prop.

```tsx
<BarChart accessibilityLayer data={chartData}>
  <CartesianGrid vertical={false} />
  <XAxis dataKey="month" />
  <Bar dataKey="desktop" fill="var(--color-desktop)" />
  <ChartTooltip content={<ChartTooltipContent />} />
</BarChart>
```

This adds:

- Keyboard arrow key navigation
- ARIA labels for chart elements
- Screen reader announcements for data values

---

### shadcn/ui Chart Component - Recharts Dependencies

Source: https://ui.shadcn.com/docs/components/chart

The chart component requires the following Recharts dependencies to be installed.

```bash
pnpm add recharts
npm install recharts
yarn add recharts
```

Recharts provides the following chart types:

- Area, Bar, Line, Pie, Composed
- Radar, RadialBar, Scatter
- Funnel, Treemap
