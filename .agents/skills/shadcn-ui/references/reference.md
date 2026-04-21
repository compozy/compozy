# shadcn.io Component Library

shadcn.io is a comprehensive React UI component library built on shadcn/ui principles, providing developers with production-ready, composable components for modern web applications. The library serves as a centralized resource for React developers who need high-quality UI components with TypeScript support, ranging from basic interactive elements to advanced AI-powered integrations. Unlike traditional component libraries that require package installations, shadcn.io components are designed to be copied directly into your project, giving you full control and customization capabilities.

The library encompasses four major categories: composable UI components (terminal, dock, credit cards, QR codes, color pickers), chart components built with Recharts, animation components with Tailwind CSS integration, and custom React hooks for state management and lifecycle operations. Each component follows best practices for accessibility, performance, and developer experience, with comprehensive TypeScript definitions and Next.js compatibility. The platform emphasizes flexibility and customization, allowing developers to modify components at the source level rather than being constrained by package APIs.

## Core Components

### Terminal Component

Interactive terminal emulator with typing animations and command execution simulation for developer-focused interfaces.

```tsx
import { Terminal } from "@/components/ui/terminal"

export default function DemoTerminal() {
 return (
 npm install @repo/terminalInstalling dependencies...npm start
 )
}
```

### Dock Component

macOS-style application dock with smooth magnification effects on hover, perfect for navigation menus.

```tsx
import { Dock, DockIcon } from "@/components/ui/dock"
import { Home, Settings, User, Mail } from "lucide-react"

export default function AppDock() {
 return (

 )
}
```

### Credit Card Component

Interactive 3D credit card component with flip animations for payment forms and card displays.

```tsx
import { CreditCard } from "@/components/ui/credit-card"
import { useState } from "react"

export default function PaymentForm() {
 const [cardData, setCardData] = useState({
 number: "4532 1234 5678 9010",
 holder: "JOHN DOE",
 expiry: "12/28",
 cvv: "123"
 })

 return (
  console.log("Card flipped:", flipped)}
 />
 )
}
```

### Image Zoom Component

Zoomable image component with smooth modal transitions for image galleries and product displays.

```tsx
import { ImageZoom } from "@/components/ui/image-zoom"

export default function ProductGallery() {
 return (


 )
}
```

### QR Code Component

Generate and display customizable QR codes with styling options for links, contact information, and authentication.

```tsx
import { QRCode } from "@/components/ui/qr-code"

export default function ShareDialog() {
 const shareUrl = "https://shadcn.io"

 return (


Scan to visit shadcn.io


 )
}
```

### Color Picker Component

Advanced color selection component supporting multiple color formats (HEX, RGB, HSL) with preview.

```tsx
import { ColorPicker } from "@/components/ui/color-picker"
import { useState } from "react"

export default function ThemeCustomizer() {
 const [color, setColor] = useState("#3b82f6")

 return (


Selected: {color}

)
}
```

## Chart Components

### Bar Chart Component

Clean bar chart component for data comparison and categorical analysis using Recharts.

```tsx
import { BarChart } from "@/components/ui/bar-chart"

export default function SalesChart() {
const data = [
{ month: "Jan", sales: 4000, revenue: 2400 },
{ month: "Feb", sales: 3000, revenue: 1398 },
{ month: "Mar", sales: 2000, revenue: 9800 },
{ month: "Apr", sales: 2780, revenue: 3908 },
{ month: "May", sales: 1890, revenue: 4800 },
{ month: "Jun", sales: 2390, revenue: 3800 }
]

return (
`$${value.toLocaleString()}`}
yAxisWidth={60}
/>
)
}
```

### Line Chart Component

Smooth line chart for visualizing trends and time-series data with multiple data series support.

```tsx
import { LineChart } from "@/components/ui/line-chart"

export default function MetricsChart() {
const data = [
{ date: "2024-01", users: 1200, sessions: 3400 },
{ date: "2024-02", users: 1800, sessions: 4200 },
{ date: "2024-03", users: 2400, sessions: 5800 },
{ date: "2024-04", users: 3100, sessions: 7200 },
{ date: "2024-05", users: 3800, sessions: 8900 }
]

return (

)
}
```

### Pie Chart Component

Donut chart component for displaying proportional data and percentage distributions.

```tsx
import { PieChart } from "@/components/ui/pie-chart"

export default function MarketShareChart() {
const data = [
{ name: "Product A", value: 400, fill: "#3b82f6" },
{ name: "Product B", value: 300, fill: "#10b981" },
{ name: "Product C", value: 300, fill: "#f59e0b" },
{ name: "Product D", value: 200, fill: "#ef4444" }
]

return (
`${entry.name}: ${entry.value}`}
/>
)
}
```

### Area Chart Component

Stacked area chart for visualizing volume changes over time with multiple data series.

```tsx
import { AreaChart } from "@/components/ui/area-chart"

export default function TrafficChart() {
const data = [
{ month: "Jan", mobile: 2000, desktop: 3000, tablet: 1000 },
{ month: "Feb", mobile: 2200, desktop: 3200, tablet: 1100 },
{ month: "Mar", mobile: 2800, desktop: 3800, tablet: 1300 },
{ month: "Apr", mobile: 3200, desktop: 4200, tablet: 1500 },
{ month: "May", mobile: 3800, desktop: 4800, tablet: 1800 }
]

return (

)
}
```

### Radar Chart Component

Multi-axis chart for comparing multiple variables across different categories simultaneously.

```tsx
import { RadarChart } from "@/components/ui/radar-chart"

export default function SkillsChart() {
const data = [
{ skill: "JavaScript", score: 85, industry: 75 },
{ skill: "TypeScript", score: 80, industry: 70 },
{ skill: "React", score: 90, industry: 80 },
{ skill: "Node.js", score: 75, industry: 72 },
{ skill: "CSS", score: 88, industry: 78 }
]

return (

)
}
```

### Mixed Chart Component

Combined bar and line chart for displaying multiple data types with different visualization methods.

```tsx
import { MixedChart } from "@/components/ui/mixed-chart"

export default function PerformanceChart() {
const data = [
{ month: "Jan", revenue: 4000, growth: 5.2 },
{ month: "Feb", revenue: 4200, growth: 5.0 },
{ month: "Mar", revenue: 4800, growth: 14.3 },
{ month: "Apr", revenue: 5200, growth: 8.3 },
{ month: "May", revenue: 5800, growth: 11.5 }
]

return (

)
}
```

## Animation Components

### Magnetic Effect Component

Magnetic hover effect that smoothly follows cursor movement for interactive buttons and cards.

```tsx
import { Magnetic } from "@/components/ui/magnetic"

export default function InteractiveButton() {
return (

Hover me

)
}
```

### Animated Cursor Component

Custom animated cursor with interactive effects and particle trails for immersive experiences.

```tsx
import { AnimatedCursor } from "@/components/ui/animated-cursor"

export default function Layout({ children }) {
return (
<>

{children}

)
}
```

### Apple Hello Effect Component

Recreation of Apple's iconic "hello" animation with multi-language text transitions.

```tsx
import { AppleHello } from "@/components/ui/apple-hello"

export default function WelcomeScreen() {
const greetings = [
{ text: "Hello", lang: "en" },
{ text: "Bonjour", lang: "fr" },
{ text: "こんにちは", lang: "ja" },
{ text: "Hola", lang: "es" },
{ text: "你好", lang: "zh" }
]

return (

)
}
```

### Liquid Button Component

Button with fluid liquid animation effect on hover for engaging call-to-action elements.

```tsx
import { LiquidButton } from "@/components/ui/liquid-button"

export default function CTASection() {
return (
console.log("CTA clicked")}
>
Get Started

)
}
```

### Rolling Text Component

Text animation that creates a rolling effect with smooth character transitions.

```tsx
import { RollingText } from "@/components/ui/rolling-text"

export default function AnimatedHeading() {
return (

)
}
```

### Shimmering Text Component

Text with animated shimmer effect for attention-grabbing headings and highlights.

```tsx
import { ShimmeringText } from "@/components/ui/shimmering-text"

export default function Hero() {
return (

)
}
```

## React Hooks

### useBoolean Hook

Enhanced boolean state management with toggle, enable, and disable methods for cleaner component logic.

```tsx
import { useBoolean } from "@/hooks/use-boolean"

export default function TogglePanel() {
const modal = useBoolean(false)
const loading = useBoolean(false)

const handleSubmit = async () => {
loading.setTrue()
try {
await submitForm()
modal.setFalse()
} finally {
loading.setFalse()
}
}

return (
<>
Toggle Modal
{modal.value && (


Status: {loading.value ? "Saving..." : "Ready"}

Submit


)}

)
}
```

### useCounter Hook

Counter hook with increment, decrement, reset, and set functionality for numeric state management.

```tsx
import { useCounter } from "@/hooks/use-counter"

export default function CartCounter() {
const quantity = useCounter(0, { min: 0, max: 99 })

return (


 -
{quantity.value}
+

Reset


)
}
```

### useLocalStorage Hook

Persist state in browser localStorage with automatic serialization and deserialization.

```tsx
import { useLocalStorage } from "@/hooks/use-local-storage"

export default function UserPreferences() {
const [theme, setTheme] = useLocalStorage("theme", "light")
const [settings, setSettings] = useLocalStorage("settings", {
notifications: true,
emailUpdates: false
})

return (


setTheme(e.target.value)}>
LightDark setSettings({
...settings,
notifications: e.target.checked
})}
/>
Enable Notifications


)
}
```

### useDebounceValue Hook

Debounce values to prevent excessive updates and API calls during rapid user input.

```tsx
import { useDebounceValue } from "@/hooks/use-debounce-value"
import { useState, useEffect } from "react"

export default function SearchBox() {
const [search, setSearch] = useState("")
const debouncedSearch = useDebounceValue(search, 500)
const [results, setResults] = useState([])
const [apiCalls, setApiCalls] = useState(0)

useEffect(() => {
if (debouncedSearch) {
setApiCalls(prev => prev + 1)
fetch(`/api/search?q=${debouncedSearch}`)
.then(res => res.json())
.then(setResults)
}
}, [debouncedSearch])

return (


setSearch(e.target.value)}
placeholder="Search..."
/>


API calls: {apiCalls}


)
}
```

### useHover Hook

Track hover state on elements with customizable enter and leave delays for tooltip and preview functionality.

```tsx
import { useHover } from "@/hooks/use-hover"
import { useRef } from "react"

export default function ImagePreview() {
const hoverRef = useRef(null)
const isHovering = useHover(hoverRef, {
enterDelay: 200,
leaveDelay: 100
})

return (


![Preview](http://https:%2F%2Fcontext7.com%2Fwebsites%2Fshadcn_io%2Fllms.txt/thumbnail.jpg)
{isHovering && (


![Full size](http://https:%2F%2Fcontext7.com%2Fwebsites%2Fshadcn_io%2Fllms.txt/full-size.jpg)

)}


)
}
```

### useCountdown Hook

Countdown timer with play, pause, reset controls and completion callbacks for time-limited features.

```tsx
import { useCountdown } from "@/hooks/use-countdown"

export default function OTPTimer() {
const countdown = useCountdown({
initialSeconds: 60,
onComplete: () => alert("OTP expired! Request a new code.")
})

return (


{countdown.seconds}s

{!countdown.isRunning ? (
Start
) : (
Pause
)}
Reset

Status: {countdown.isComplete ? "Expired" : countdown.isRunning ? "Active" : "Paused"}


)
}
```

## Installation and Usage

### CLI Installation

Install components directly into your project using the shadcn CLI for instant integration.

```bash
# Initialize shadcn in your project
npx shadcn@latest init

# Add individual components
npx shadcn@latest add terminal
npx shadcn@latest add dock
npx shadcn@latest add credit-card

# Add multiple components at once
npx shadcn@latest add bar-chart line-chart pie-chart

# Add hooks
npx shadcn@latest add use-boolean use-counter use-local-storage
```

### Project Configuration

Configure your project to work with shadcn.io components using TypeScript and Tailwind CSS.

```typescript
// tailwind.config.ts
import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./pages/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./app/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};

export default config;
```

## Summary

The shadcn.io component library serves as a comprehensive toolkit for React developers building modern web applications with Next.js and TypeScript. The library's primary use cases include rapid prototyping of user interfaces, building data-rich dashboards with interactive charts, creating engaging user experiences with animations and effects, and implementing common UI patterns without writing boilerplate code. The copy-paste approach gives developers complete ownership of their components, allowing for deep customization while maintaining consistency with shadcn/ui design principles. Components are particularly well-suited for SaaS applications, admin panels, marketing websites, and e-commerce platforms that require professional, accessible UI elements.

Integration patterns center around composability and customization rather than rigid package dependencies. Developers can cherry-pick individual components using the CLI, modify them at the source level to match their design system, and combine them with existing shadcn/ui components for a cohesive interface. The library supports both light and dark themes through CSS variables, integrates seamlessly with Tailwind CSS utility classes, and follows React best practices for performance and accessibility. Custom hooks provide reusable logic patterns that complement the visual components, creating a complete ecosystem for building feature-rich applications. The TypeScript-first approach ensures type safety throughout the development process, while the Recharts integration for data visualization provides powerful charting capabilities without additional configuration overhead.
