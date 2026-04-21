---
name: shadcn-ui
description: Provides complete shadcn/ui component library patterns including installation, configuration, and implementation of accessible React components. Use when setting up shadcn/ui, installing components, building forms with React Hook Form and Zod, customizing themes with Tailwind CSS, or implementing UI patterns like buttons, dialogs, dropdowns, tables, and complex form layouts.
allowed-tools: Read, Write, Bash, Edit, Glob
---

# shadcn/ui Component Patterns

## Overview

Expert guide for building accessible, customizable UI components with shadcn/ui, Radix UI, and Tailwind CSS. This skill provides comprehensive patterns for implementing production-ready components with full accessibility support.

## Table of Contents

- [When to Use](#when-to-use)
- [Quick Start](#quick-start)
- [Installation & Setup](#installation--setup)
- [Project Configuration](#project-configuration)
- [Core Components](#core-components)
  - [Button](#button-component)
  - [Input & Form Fields](#input--form-fields)
  - [Forms with Validation](#forms-with-validation)
  - [Card](#card-component)
  - [Dialog (Modal)](#dialog-modal-component)
  - [Select (Dropdown)](#select-dropdown-component)
  - [Sheet (Slide-over)](#sheet-slide-over-component)
  - [Menubar & Navigation](#menubar--navigation)
  - [Table](#table-component)
  - [Toast Notifications](#toast-notifications)
  - [Charts](#charts-component)
- [Advanced Patterns](#advanced-patterns)
- [Customization](#customization)
- [Next.js Integration](#nextjs-integration)
- [Best Practices](#best-practices)
- [Common Component Combinations](#common-component-combinations)

## When to Use

- Setting up a new project with shadcn/ui
- Installing or configuring individual components
- Building forms with React Hook Form and Zod validation
- Creating accessible UI components (buttons, dialogs, dropdowns, sheets)
- Customizing component styling with Tailwind CSS
- Implementing design systems with shadcn/ui
- Building Next.js applications with TypeScript
- Creating complex layouts and data displays

## Instructions

1. **Initialize Project**: Run `npx shadcn@latest init` to configure shadcn/ui
2. **Install Components**: Add components with `npx shadcn@latest add <component>`
3. **Configure Theme**: Customize CSS variables in globals.css for theming
4. **Import Components**: Use components from `@/components/ui/` directory
5. **Customize as Needed**: Modify component code directly in your project
6. **Add Form Validation**: Integrate React Hook Form with Zod schemas
7. **Test Accessibility**: Verify ARIA attributes and keyboard navigation

## Examples

### Complete Form with Validation

```tsx
"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";

const formSchema = z.object({
  email: z.string().email("Invalid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
});

export function LoginForm() {
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { email: "", password: "" },
  });

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(console.log)} className="space-y-4">
        <FormField
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input type="email" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button type="submit">Login</Button>
      </form>
    </Form>
  );
}
```

## Constraints and Warnings

- **Not an NPM Package**: Components are copied to your project; you own the code
- **Client Components**: Most components require "use client" directive
- **Radix Dependencies**: Ensure all @radix-ui packages are installed
- **Tailwind Required**: Components rely on Tailwind CSS utilities
- **TypeScript**: Designed for TypeScript projects; type definitions included
- **Path Aliases**: Configure @ alias in tsconfig.json for imports
- **Dark Mode**: Set up dark mode with CSS variables or class strategy

## Quick Start

For new projects, use the automated setup:

```bash
# Create Next.js project with shadcn/ui
npx create-next-app@latest my-app --typescript --tailwind --eslint --app
cd my-app
npx shadcn@latest init

# Install essential components
npx shadcn@latest add button input form card dialog select
```

For existing projects:

```bash
# Install dependencies
npm install tailwindcss-animate class-variance-authority clsx tailwind-merge lucide-react

# Initialize shadcn/ui
npx shadcn@latest init
```

## What is shadcn/ui?

shadcn/ui is **not** a traditional component library or npm package. Instead:

- It's a **collection of reusable components** that you can copy into your project
- Components are **yours to customize** - you own the code
- Built with **Radix UI** primitives for accessibility
- Styled with **Tailwind CSS** utilities
- Includes CLI tool for easy component installation

## Installation & Setup

### Initial Setup

```bash
# Initialize shadcn/ui in your project
npx shadcn@latest init
```

During setup, you'll configure:

- TypeScript or JavaScript
- Style (Default, New York, etc.)
- Base color theme
- CSS variables or Tailwind CSS classes
- Component installation path

### Installing Individual Components

```bash
# Install a single component
npx shadcn@latest add button

# Install multiple components
npx shadcn@latest add button input form

# Install all components
npx shadcn@latest add --all
```

### Manual Installation

If you prefer manual setup:

```bash
# Install dependencies for a specific component
npm install @radix-ui/react-slot

# Copy component code from ui.shadcn.com
# Place in src/components/ui/
```

## Project Configuration

### Required Dependencies

```json
{
  "dependencies": {
    "@radix-ui/react-accordion": "^1.1.2",
    "@radix-ui/react-alert-dialog": "^1.0.5",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-dropdown-menu": "^2.0.6",
    "@radix-ui/react-label": "^2.0.2",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-separator": "^1.0.3",
    "@radix-ui/react-slot": "^1.0.2",
    "@radix-ui/react-toast": "^1.1.5",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.0.0",
    "lucide-react": "^0.294.0",
    "tailwind-merge": "^2.0.0",
    "tailwindcss-animate": "^1.0.7"
  }
}
```

### TSConfig Configuration

```json
{
  "compilerOptions": {
    "target": "es5",
    "lib": ["dom", "dom.iterable", "es6"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "preserve",
    "incremental": true,
    "plugins": [
      {
        "name": "next"
      }
    ],
    "baseUrl": ".",
    "paths": {
      "@/components/*": ["./src/components/*"],
      "@/lib/*": ["./src/lib/*"]
    }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}
```

### Tailwind Configuration

```js
// tailwind.config.js
/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: ["class"],
  content: [
    "./pages/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./app/**/*.{ts,tsx}",
    "./src/**/*.{ts,tsx}",
  ],
  prefix: "",
  theme: {
    container: {
      center: true,
      padding: "2rem",
      screens: {
        "2xl": "1400px",
      },
    },
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
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
      keyframes: {
        "accordion-down": {
          from: { height: "0" },
          to: { height: "var(--radix-accordion-content-height)" },
        },
        "accordion-up": {
          from: { height: "var(--radix-accordion-content-height)" },
          to: { height: "0" },
        },
      },
      animation: {
        "accordion-down": "accordion-down 0.2s ease-out",
        "accordion-up": "accordion-up 0.2s ease-out",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};
```

### CSS Variables (globals.css)

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;
    --popover: 0 0% 100%;
    --popover-foreground: 222.2 84% 4.9%;
    --primary: 222.2 47.4% 11.2%;
    --primary-foreground: 210 40% 98%;
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;
    --border: 214.3 31.8% 91.4%;
    --input: 214.3 31.8% 91.4%;
    --ring: 222.2 84% 4.9%;
    --radius: 0.5rem;
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --card: 222.2 84% 4.9%;
    --card-foreground: 210 40% 98%;
    --popover: 222.2 84% 4.9%;
    --popover-foreground: 210 40% 98%;
    --primary: 210 40% 98%;
    --primary-foreground: 222.2 47.4% 11.2%;
    --secondary: 217.2 32.6% 17.5%;
    --secondary-foreground: 210 40% 98%;
    --muted: 217.2 32.6% 17.5%;
    --muted-foreground: 215 20.2% 65.1%;
    --accent: 217.2 32.6% 17.5%;
    --accent-foreground: 210 40% 98%;
    --destructive: 0 62.8% 30.6%;
    --destructive-foreground: 210 40% 98%;
    --border: 217.2 32.6% 17.5%;
    --input: 217.2 32.6% 17.5%;
    --ring: 212.7 26.8% 83.9%;
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
  }
}
```

## Core Components

### Button Component

Installation:

```bash
npx shadcn@latest add button
```

Basic usage:

```tsx
import { Button } from "@/components/ui/button";

export function ButtonDemo() {
  return <Button>Click me</Button>;
}
```

Button variants:

```tsx
import { Button } from "@/components/ui/button";

export function ButtonVariants() {
  return (
    <div className="flex gap-4">
      <Button variant="default">Default</Button>
      <Button variant="destructive">Destructive</Button>
      <Button variant="outline">Outline</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="ghost">Ghost</Button>
      <Button variant="link">Link</Button>
    </div>
  );
}
```

Button sizes:

```tsx
<div className="flex gap-4 items-center">
  <Button size="default">Default</Button>
  <Button size="sm">Small</Button>
  <Button size="lg">Large</Button>
  <Button size="icon">
    <Icon className="h-4 w-4" />
  </Button>
</div>
```

With loading state:

```tsx
import { Button } from "@/components/ui/button";
import { Loader2 } from "lucide-react";

export function ButtonLoading() {
  return (
    <Button disabled>
      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
      Please wait
    </Button>
  );
}
```

### Input & Form Fields

#### Input Component

Installation:

```bash
npx shadcn@latest add input
```

Basic input:

```tsx
import { Input } from "@/components/ui/input";

export function InputDemo() {
  return <Input type="email" placeholder="Email" />;
}
```

Input with label:

```tsx
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function InputWithLabel() {
  return (
    <div className="grid w-full max-w-sm items-center gap-1.5">
      <Label htmlFor="email">Email</Label>
      <Input type="email" id="email" placeholder="Email" />
    </div>
  );
}
```

Input with button:

```tsx
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function InputWithButton() {
  return (
    <div className="flex w-full max-w-sm items-center gap-2">
      <Input type="email" placeholder="Email" />
      <Button type="submit" variant="outline">
        Subscribe
      </Button>
    </div>
  );
}
```

### Forms with Validation

Installation:

```bash
npx shadcn@latest add form
```

This installs React Hook Form, Zod, and form components.

Complete form example:

```tsx
"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import * as z from "zod";

import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/use-toast";

const formSchema = z.object({
  username: z.string().min(2, {
    message: "Username must be at least 2 characters.",
  }),
  email: z.string().email({
    message: "Please enter a valid email address.",
  }),
});

export function ProfileForm() {
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      username: "",
      email: "",
    },
  });

  function onSubmit(values: z.infer<typeof formSchema>) {
    toast({
      title: "You submitted the following values:",
      description: (
        <pre className="mt-2 w-[340px] rounded-md bg-slate-950 p-4">
          <code className="text-white">{JSON.stringify(values, null, 2)}</code>
        </pre>
      ),
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
        <FormField
          control={form.control}
          name="username"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Username</FormLabel>
              <FormControl>
                <Input placeholder="shadcn" {...field} />
              </FormControl>
              <FormDescription>This is your public display name.</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input type="email" placeholder="you@example.com" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button type="submit">Submit</Button>
      </form>
    </Form>
  );
}
```

### Card Component

Installation:

```bash
npx shadcn@latest add card
```

Basic card:

```tsx
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function CardDemo() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Card Title</CardTitle>
        <CardDescription>Card Description</CardDescription>
      </CardHeader>
      <CardContent>
        <p>Card Content</p>
      </CardContent>
      <CardFooter>
        <p>Card Footer</p>
      </CardFooter>
    </Card>
  );
}
```

Card with form:

```tsx
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function CardWithForm() {
  return (
    <Card className="w-[350px]">
      <CardHeader>
        <CardTitle>Create project</CardTitle>
        <CardDescription>Deploy your new project in one-click.</CardDescription>
      </CardHeader>
      <CardContent>
        <form>
          <div className="grid w-full items-center gap-4">
            <div className="flex flex-col space-y-1.5">
              <Label htmlFor="name">Name</Label>
              <Input id="name" placeholder="Name of your project" />
            </div>
          </div>
        </form>
      </CardContent>
      <CardFooter className="flex justify-between">
        <Button variant="outline">Cancel</Button>
        <Button>Deploy</Button>
      </CardFooter>
    </Card>
  );
}
```

### Dialog (Modal) Component

Installation:

```bash
npx shadcn@latest add dialog
```

Basic dialog:

```tsx
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

export function DialogDemo() {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline">Open Dialog</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Edit profile</DialogTitle>
          <DialogDescription>
            Make changes to your profile here. Click save when you're done.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-4">
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="name" className="text-right">
              Name
            </Label>
            <Input id="name" value="Pedro Duarte" className="col-span-3" />
          </div>
        </div>
        <DialogFooter>
          <Button type="submit">Save changes</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

### Sheet (Slide-over) Component

Installation:

```bash
npx shadcn@latest add sheet
```

Basic sheet:

```tsx
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

export function SheetDemo() {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="outline">Open Sheet</Button>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Edit profile</SheetTitle>
          <SheetDescription>
            Make changes to your profile here. Click save when you're done.
          </SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 py-4">
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="name" className="text-right">
              Name
            </Label>
            <Input id="name" value="Pedro Duarte" className="col-span-3" />
          </div>
          <div className="grid grid-cols-4 items-center gap-4">
            <Label htmlFor="username" className="text-right">
              Username
            </Label>
            <Input id="username" value="@peduarte" className="col-span-3" />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
```

Sheet with side placement:

```tsx
<Sheet>
  <SheetTrigger asChild>
    <Button variant="outline">Open Right Sheet</Button>
  </SheetTrigger>
  <SheetContent side="right">
    <SheetHeader>
      <SheetTitle>Settings</SheetTitle>
      <SheetDescription>Configure your application settings here.</SheetDescription>
    </SheetHeader>
    {/* Settings content */}
  </SheetContent>
</Sheet>
```

### Menubar & Navigation

#### Menubar Component

Installation:

```bash
npx shadcn@latest add menubar
```

Basic menubar:

```tsx
import {
  Menubar,
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "@/components/ui/menubar";

export function MenubarDemo() {
  return (
    <Menubar>
      <MenubarMenu>
        <MenubarTrigger>File</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>
            New Tab <MenubarShortcut>⌘T</MenubarShortcut>
          </MenubarItem>
          <MenubarItem>
            New Window <MenubarShortcut>⌘N</MenubarShortcut>
          </MenubarItem>
          <MenubarSeparator />
          <MenubarItem>Share</MenubarItem>
          <MenubarSeparator />
          <MenubarItem>Print</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Edit</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>
            Undo <MenubarShortcut>⌘Z</MenubarShortcut>
          </MenubarItem>
          <MenubarItem>
            Redo <MenubarShortcut>⌘Y</MenubarShortcut>
          </MenubarItem>
          <MenubarSeparator />
          <MenubarSub>
            <MenubarSubTrigger>Find</MenubarSubTrigger>
            <MenubarSubContent>
              <MenubarItem>Search the web</MenubarItem>
              <MenubarItem>Find...</MenubarItem>
              <MenubarItem>Find Next</MenubarItem>
              <MenubarItem>Find Previous</MenubarItem>
            </MenubarSubContent>
          </MenubarSub>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  );
}
```

### Select (Dropdown) Component

Installation:

```bash
npx shadcn@latest add select
```

Basic select:

```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function SelectDemo() {
  return (
    <Select>
      <SelectTrigger className="w-[180px]">
        <SelectValue placeholder="Select a fruit" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="apple">Apple</SelectItem>
        <SelectItem value="banana">Banana</SelectItem>
        <SelectItem value="orange">Orange</SelectItem>
      </SelectContent>
    </Select>
  );
}
```

Select in form:

```tsx
<FormField
  control={form.control}
  name="role"
  render={({ field }) => (
    <FormItem>
      <FormLabel>Role</FormLabel>
      <Select onValueChange={field.onChange} defaultValue={field.value}>
        <FormControl>
          <SelectTrigger>
            <SelectValue placeholder="Select a role" />
          </SelectTrigger>
        </FormControl>
        <SelectContent>
          <SelectItem value="admin">Admin</SelectItem>
          <SelectItem value="user">User</SelectItem>
          <SelectItem value="guest">Guest</SelectItem>
        </SelectContent>
      </Select>
      <FormMessage />
    </FormItem>
  )}
/>
```

### Toast Notifications

Installation:

```bash
npx shadcn@latest add toast
```

Setup toast provider in root layout:

```tsx
import { Toaster } from "@/components/ui/toaster";

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        {children}
        <Toaster />
      </body>
    </html>
  );
}
```

Using toast:

```tsx
import { useToast } from "@/components/ui/use-toast";
import { Button } from "@/components/ui/button";

export function ToastDemo() {
  const { toast } = useToast();

  return (
    <Button
      onClick={() => {
        toast({
          title: "Scheduled: Catch up",
          description: "Friday, February 10, 2023 at 5:57 PM",
        });
      }}
    >
      Show Toast
    </Button>
  );
}
```

Toast variants:

```tsx
// Success
toast({
  title: "Success",
  description: "Your changes have been saved.",
});

// Error
toast({
  variant: "destructive",
  title: "Error",
  description: "Something went wrong.",
});

// With action
toast({
  title: "Uh oh! Something went wrong.",
  description: "There was a problem with your request.",
  action: <ToastAction altText="Try again">Try again</ToastAction>,
});
```

### Table Component

Installation:

```bash
npx shadcn@latest add table
```

Basic table:

```tsx
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const invoices = [
  { invoice: "INV001", status: "Paid", method: "Credit Card", amount: "$250.00" },
  { invoice: "INV002", status: "Pending", method: "PayPal", amount: "$150.00" },
];

export function TableDemo() {
  return (
    <Table>
      <TableCaption>A list of your recent invoices.</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Invoice</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Method</TableHead>
          <TableHead className="text-right">Amount</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {invoices.map(invoice => (
          <TableRow key={invoice.invoice}>
            <TableCell className="font-medium">{invoice.invoice}</TableCell>
            <TableCell>{invoice.status}</TableCell>
            <TableCell>{invoice.method}</TableCell>
            <TableCell className="text-right">{invoice.amount}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

### Charts Component

Installation:

```bash
npx shadcn@latest add chart
```

The charts component in shadcn/ui is built on **Recharts** - providing direct access to all Recharts capabilities with consistent theming and styling.

#### ChartContainer and ChartConfig

The `ChartContainer` wraps your Recharts component and accepts a `config` prop for theming:

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
        <XAxis
          dataKey="month"
          tickLine={false}
          axisLine={false}
          tickFormatter={value => value.slice(0, 3)}
        />
        <Bar dataKey="desktop" fill="var(--color-desktop)" radius={4} />
        <Bar dataKey="mobile" fill="var(--color-mobile)" radius={4} />
        <ChartTooltip content={<ChartTooltipContent />} />
      </BarChart>
    </ChartContainer>
  );
}
```

#### ChartConfig with Custom Colors

You can define custom colors directly in the configuration:

```tsx
const chartConfig = {
  visitors: {
    label: "Visitors",
    color: "#2563eb", // Custom hex color
    theme: {
      light: "#2563eb",
      dark: "#60a5fa",
    },
  },
  sales: {
    label: "Sales",
    color: "var(--chart-1)", // CSS variable
    theme: {
      light: "oklch(0.646 0.222 41.116)",
      dark: "oklch(0.696 0.182 281.41)",
    },
  },
} satisfies import("@/components/ui/chart").ChartConfig;
```

#### CSS Variables for Charts

Add chart color variables to your `globals.css`:

```css
@layer base {
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
}
```

#### Line Chart Example

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

#### Area Chart Example

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

#### Pie Chart Example

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

#### ChartTooltipContent Props

| Prop            | Type                        | Default | Description           |
| --------------- | --------------------------- | ------- | --------------------- |
| `labelKey`      | string                      | "label" | Key for tooltip label |
| `nameKey`       | string                      | "name"  | Key for tooltip name  |
| `indicator`     | "dot" \| "line" \| "dashed" | "dot"   | Indicator style       |
| `hideLabel`     | boolean                     | false   | Hide label            |
| `hideIndicator` | boolean                     | false   | Hide indicator        |

#### Accessibility

Enable keyboard navigation and screen reader support:

```tsx
<BarChart accessibilityLayer data={chartData}>
  ...
</BarChart>
```

This adds:

- Keyboard arrow key navigation
- ARIA labels for chart elements
- Screen reader announcements for data values

## Customization

### Theming with CSS Variables

shadcn/ui uses CSS variables for theming. Configure in `globals.css`:

```css
@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --primary: 222.2 47.4% 11.2%;
    --primary-foreground: 210 40% 98%;
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;
    --border: 214.3 31.8% 91.4%;
    --input: 214.3 31.8% 91.4%;
    --ring: 222.2 84% 4.9%;
    --radius: 0.5rem;
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --primary: 210 40% 98%;
    --primary-foreground: 222.2 47.4% 11.2%;
    /* ... other dark mode variables */
  }
}
```

### Customizing Components

Since you own the code, customize directly:

```tsx
// components/ui/button.tsx
import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
        outline: "border border-input bg-background hover:bg-accent",
        // Add custom variant
        custom: "bg-gradient-to-r from-purple-500 to-pink-500 text-white",
      },
      size: {
        default: "h-10 px-4 py-2",
        sm: "h-9 rounded-md px-3",
        lg: "h-11 rounded-md px-8",
        // Add custom size
        xl: "h-14 rounded-md px-10 text-lg",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
```

## Next.js Integration

### App Router Setup

For Next.js 13+ with App Router, ensure components use `"use client"` directive:

```tsx
// src/components/ui/button.tsx
"use client";

import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

// ... rest of component
```

### Layout Integration

Add the Toaster to your root layout:

```tsx
// app/layout.tsx
import { Toaster } from "@/components/ui/toaster";
import "./globals.css";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="min-h-screen bg-background font-sans antialiased">
        {children}
        <Toaster />
      </body>
    </html>
  );
}
```

### Server Components

When using shadcn/ui components in Server Components, wrap them in a Client Component:

```tsx
// app/dashboard/page.tsx
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ButtonClient } from "@/components/ui/button-client";

export default function DashboardPage() {
  return (
    <div className="container mx-auto p-6">
      <Card>
        <CardHeader>
          <CardTitle>Dashboard</CardTitle>
        </CardHeader>
        <CardContent>
          <ButtonClient>Interactive Button</ButtonClient>
        </CardContent>
      </Card>
    </div>
  );
}
```

```tsx
// src/components/ui/button-client.tsx
"use client";

import { Button } from "./button";

export function ButtonClient(props: React.ComponentProps<typeof Button>) {
  return <Button {...props} />;
}
```

### Route Handlers with Forms

Create API routes for form submissions:

```tsx
// app/api/contact/route.ts
import { NextRequest, NextResponse } from "next/server";
import { z } from "zod";

const contactSchema = z.object({
  name: z.string().min(2),
  email: z.string().email(),
  message: z.string().min(10),
});

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const validated = contactSchema.parse(body);

    // Process form data
    console.log("Form submission:", validated);

    return NextResponse.json({ success: true });
  } catch (error) {
    if (error instanceof z.ZodError) {
      return NextResponse.json({ errors: error.errors }, { status: 400 });
    }

    return NextResponse.json({ error: "Internal server error" }, { status: 500 });
  }
}
```

### Form with Server Action

Using Next.js 14+ Server Actions:

```tsx
// app/contact/page.tsx
"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import * as z from "zod";
import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/use-toast";

const formSchema = z.object({
  name: z.string().min(2),
  email: z.string().email(),
  message: z.string().min(10),
});

async function onSubmit(values: z.infer<typeof formSchema>) {
  try {
    const response = await fetch("/api/contact", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(values),
    });

    if (!response.ok) throw new Error("Failed to submit");

    toast({
      title: "Success!",
      description: "Your message has been sent.",
    });
  } catch (error) {
    toast({
      variant: "destructive",
      title: "Error",
      description: "Failed to send message. Please try again.",
    });
  }
}

export default function ContactPage() {
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
  });

  return (
    <div className="container mx-auto max-w-2xl py-8">
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Name</FormLabel>
                <FormControl>
                  <Input placeholder="Your name" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Email</FormLabel>
                <FormControl>
                  <Input type="email" placeholder="your@email.com" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="message"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Message</FormLabel>
                <FormControl>
                  <Textarea placeholder="Your message..." className="resize-none" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full">
            Send Message
          </Button>
        </form>
      </Form>
    </div>
  );
}
```

### Metadata with shadcn/ui

Using shadcn/ui components in metadata:

```tsx
// app/layout.tsx
import { Metadata } from "next";

export const metadata: Metadata = {
  title: {
    default: "My App",
    template: "%s | My App",
  },
  description: "Built with shadcn/ui and Next.js",
};

// app/about/page.tsx
import { Metadata } from "next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export const metadata: Metadata = {
  title: "About Us",
  description: "Learn more about our company",
};

export default function AboutPage() {
  return (
    <div className="container mx-auto py-8">
      <Card>
        <CardHeader>
          <CardTitle>About Our Company</CardTitle>
        </CardHeader>
        <CardContent>
          <p>We build amazing products with modern web technologies.</p>
        </CardContent>
      </Card>
    </div>
  );
}
```

### Font Optimization

Optimize fonts with next/font:

```tsx
// app/layout.tsx
import { Inter } from "next/font/google";
import { Toaster } from "@/components/ui/toaster";
import { cn } from "@/lib/utils";
import "./globals.css";

const inter = Inter({ subsets: ["latin"] });

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={cn("min-h-screen bg-background font-sans antialiased", inter.className)}>
        {children}
        <Toaster />
      </body>
    </html>
  );
}
```

## Advanced Patterns

### Form with Multiple Fields

```tsx
const formSchema = z.object({
  username: z.string().min(2).max(50),
  email: z.string().email(),
  bio: z.string().max(160).min(4),
  role: z.enum(["admin", "user", "guest"]),
  notifications: z.boolean().default(false),
});

export function AdvancedForm() {
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      username: "",
      email: "",
      bio: "",
      role: "user",
      notifications: false,
    },
  });

  function onSubmit(values: z.infer<typeof formSchema>) {
    console.log(values);
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
        {/* Username field */}
        <FormField
          control={form.control}
          name="username"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Username</FormLabel>
              <FormControl>
                <Input placeholder="johndoe" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Email field */}
        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input type="email" placeholder="john@example.com" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Textarea field */}
        <FormField
          control={form.control}
          name="bio"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Bio</FormLabel>
              <FormControl>
                <Textarea placeholder="Tell us about yourself" className="resize-none" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Select field */}
        <FormField
          control={form.control}
          name="role"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Role</FormLabel>
              <Select onValueChange={field.onChange} defaultValue={field.value}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a role" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="user">User</SelectItem>
                  <SelectItem value="guest">Guest</SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Checkbox field */}
        <FormField
          control={form.control}
          name="notifications"
          render={({ field }) => (
            <FormItem className="flex flex-row items-start space-x-3 space-y-0">
              <FormControl>
                <Checkbox checked={field.value} onCheckedChange={field.onChange} />
              </FormControl>
              <div className="space-y-1 leading-none">
                <FormLabel>Email notifications</FormLabel>
                <FormDescription>Receive emails about your account activity.</FormDescription>
              </div>
            </FormItem>
          )}
        />

        <Button type="submit">Submit</Button>
      </form>
    </Form>
  );
}
```

## Best Practices

1. **Accessibility**: Components use Radix UI primitives for ARIA compliance
2. **Customization**: Modify components directly in your codebase
3. **Type Safety**: Use TypeScript for type-safe props and state
4. **Validation**: Use Zod schemas for form validation
5. **Styling**: Leverage Tailwind utilities and CSS variables
6. **Consistency**: Use the same component patterns across your app
7. **Testing**: Components are testable with React Testing Library
8. **Performance**: Components are optimized and tree-shakeable

## Common Component Combinations

### Login Form

```tsx
<Card className="w-[350px]">
  <CardHeader>
    <CardTitle>Login</CardTitle>
    <CardDescription>Enter your credentials to continue</CardDescription>
  </CardHeader>
  <CardContent>
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input type="email" placeholder="you@example.com" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Password</FormLabel>
              <FormControl>
                <Input type="password" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button type="submit" className="w-full">
          Login
        </Button>
      </form>
    </Form>
  </CardContent>
</Card>
```

## References

- Official Docs: https://ui.shadcn.com
- Radix UI: https://www.radix-ui.com
- React Hook Form: https://react-hook-form.com
- Zod: https://zod.dev
- Tailwind CSS: https://tailwindcss.com
- Examples: https://ui.shadcn.com/examples
