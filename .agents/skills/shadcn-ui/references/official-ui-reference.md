### Create TanStack Start Project with shadcn/ui

Source: https://ui.shadcn.com/docs/installation/tanstack

Initialize a new TanStack Start project with Tailwind CSS and shadcn/ui add-ons pre-configured. This command sets up the project structure and installs necessary dependencies in one command.

```bash
npm create @tanstack/start@latest --tailwind --add-ons shadcn
```

---

### Install All shadcn/ui Components

Source: https://ui.shadcn.com/docs/installation/tanstack

Bulk install all available shadcn/ui components into your project at once. This is useful when you want access to the entire component library without adding components individually.

```bash
npx shadcn@latest add --all
```

---

### Manually Install Radix UI Select Dependency

Source: https://ui.shadcn.com/docs/components/select

This command shows how to install the core `@radix-ui/react-select` primitive package. This manual installation is necessary if you prefer not to use the Shadcn UI CLI for component setup.

```bash
npm install @radix-ui/react-select
```

---

### Install Progress Component Dependencies

Source: https://ui.shadcn.com/docs/components/progress

This section provides instructions for installing the `Progress` component and its core dependencies. It covers both using the Shadcn UI CLI for automated setup and manual installation via npm for the underlying Radix UI component.

```bash
npx shadcn@latest add progress
```

```bash
npm install @radix-ui/react-progress
```

---

### Serve shadcn Registry with Next.js Development Server

Source: https://ui.shadcn.com/docs/registry/getting-started

This command starts the Next.js development server, which will serve your shadcn registry files if your project is configured with Next.js. The registry items will be accessible via specific URLs under `/r/` after the build process.

```bash
npm run dev
```

---

### Install shadcn CLI via npm

Source: https://ui.shadcn.com/docs/registry/getting-started

This command installs the latest version of the shadcn command-line interface (CLI) globally or as a dev dependency in your project. The CLI is essential for building and managing shadcn component registries and components.

```bash
npm install shadcn@latest
```

---

### Create New Laravel Project with React

Source: https://ui.shadcn.com/docs/installation/laravel

Initialize a new Laravel project with Inertia and React using the Laravel installer. This command creates a fresh Laravel application with React pre-configured for use with Inertia.js.

```bash
laravel new my-app --react
```

---

### Install Shadcn UI Input OTP Component (CLI & Manual)

Source: https://ui.shadcn.com/docs/components/input-otp

Provides instructions for adding the Input OTP component to a project. Users can choose between the Shadcn UI CLI for automated setup or manual installation by adding the core `input-otp` dependency via npm and then integrating the component files.

```bash
npx shadcn@latest add input-otp
```

```bash
npm install input-otp
```

---

### Install Aspect Ratio Component via CLI

Source: https://ui.shadcn.com/docs/components/aspect-ratio

Installs the aspect-ratio component and its dependencies using the shadcn CLI. This is the quickest installation method that automatically handles dependency installation and file setup.

```bash
npx shadcn@latest add aspect-ratio
```

---

### Install Dropdown Menu Component with NPM

Source: https://ui.shadcn.com/docs/components/dropdown-menu

Installation command for adding the dropdown menu component to a project using shadcn/ui CLI tool. This is the recommended method for quick setup with automatic dependency management.

```bash
npx shadcn@latest add dropdown-menu
```

---

### Define Universal Registry Item for Multi-File Template (shadcn/ui)

Source: https://ui.shadcn.com/docs/registry/examples

This JSON configuration defines a shadcn/ui registry item named 'my-custom-start-template' that installs multiple files. It includes two files, each with an explicit target path, demonstrating how to create a universal starter template that can be installed without framework detection or components.json.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "my-custom-start-template",
  "type": "registry:item",
  "dependencies": ["better-auth"],
  "files": [
    {
      "path": "/path/to/file-01.json",
      "type": "registry:file",
      "target": "~/file-01.json",
      "content": "..."
    },
    {
      "path": "/path/to/file-02.vue",
      "type": "registry:file",
      "target": "~/pages/file-02.vue",
      "content": "..."
    }
  ]
}
```

---

### Add shadcn/ui Button Component

Source: https://ui.shadcn.com/docs/installation/tanstack

Install the Button component from shadcn/ui into your TanStack Start project. This command downloads and configures the component in your project's component directory.

```bash
npx shadcn@latest add button
```

---

### Install Form Component via Shadcn CLI

Source: https://ui.shadcn.com/docs/components/form

This command provides the recommended method for installing the Shadcn UI form component using its command-line interface. Executing this command automates the addition of the form component and its dependencies to your project, simplifying the setup process.

```bash
npx shadcn@latest add form
```

---

### Basic Navigation Menu Setup - React TSX

Source: https://ui.shadcn.com/docs/components/navigation-menu

Minimal example demonstrating the basic structure of a Navigation Menu with one menu item, trigger, and content link. Serves as a foundation for more complex navigation patterns.

```typescript
Item OneLink
```

---

### Multiple Registry Setup with Mixed Authentication

Source: https://ui.shadcn.com/docs/components-json

Complete example showing how to configure multiple registries with different authentication methods and parameters. Demonstrates public registries, private registries with bearer tokens, and team registries with versioning and environment variables.

```json
{
  "registries": {
    "@shadcn": "https://ui.shadcn.com/r/{name}.json",
    "@company-ui": {
      "url": "https://registry.company.com/ui/{name}.json",
      "headers": {
        "Authorization": "Bearer ${COMPANY_TOKEN}"
      }
    },
    "@team": {
      "url": "https://team.company.com/{name}.json",
      "params": {
        "team": "frontend",
        "version": "${REGISTRY_VERSION}"
      }
    }
  }
}
```

---

### Add Component Definition to shadcn registry.json

Source: https://ui.shadcn.com/docs/registry/getting-started

This JSON snippet shows how to register a component, like `hello-world`, within the `registry.json` file. It includes metadata such as name, type, title, description, and defines the component's file path and type, ensuring it conforms to the registry item schema.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry.json",
  "name": "acme",
  "homepage": "https://acme.com",
  "items": [
    {
      "name": "hello-world",
      "type": "registry:block",
      "title": "Hello World",
      "description": "A simple hello world component.",
      "files": [
        {
          "path": "registry/new-york/hello-world/hello-world.tsx",
          "type": "registry:component"
        }
      ]
    }
  ]
}
```

---

### Install Project Dependencies using npm

Source: https://ui.shadcn.com/docs/installation/manual

This bash command installs a set of essential npm packages for the project. These dependencies include utilities for styling (`class-variance-authority`, `clsx`, `tailwind-merge`), icon library (`lucide-react`), and animation effects (`tw-animate-css`).

```bash
npm install class-variance-authority clsx tailwind-merge lucide-react tw-animate-css
```

---

### Install React Resizable Panels Dependency Manually

Source: https://ui.shadcn.com/docs/components/resizable

This `npm` command installs the core `react-resizable-panels` library, which the `Resizable` component is built upon. It is a prerequisite for manual setup and provides the underlying functionality for resizable UI elements.

```bash
npm install react-resizable-panels
```

---

### Install Shadcn UI Skeleton component using CLI

Source: https://ui.shadcn.com/docs/components/skeleton

Provides the command-line instruction to add the `Skeleton` component to your project if you are using Shadcn UI's CLI. This automates the setup process for the component.

```bash
npx shadcn@latest add skeleton
```

---

### Install Dependencies with pnpm

Source: https://ui.shadcn.com/docs/blocks

Installs project dependencies using pnpm package manager. Required before starting development on the block.

```bash
pnpm install
```

---

### Install Pagination Component - Bash CLI

Source: https://ui.shadcn.com/docs/components/pagination

Command-line installation of the pagination component using the shadcn CLI tool. This is the recommended installation method for projects using shadcn/ui.

```bash
npx shadcn@latest add pagination
```

---

### Install Sonner Dependencies Manually

Source: https://ui.shadcn.com/docs/components/sonner

Manual installation command that installs Sonner and next-themes packages required for manual setup. Use this approach when you prefer to manually configure the component instead of using the CLI.

```bash
npm install sonner next-themes
```

---

### Install Radix UI Separator Dependency via npm

Source: https://ui.shadcn.com/docs/components/separator

Install the core Radix UI React Separator dependency required for manual setup. Use this command when manually installing the component instead of using the CLI.

```bash
npm install @radix-ui/react-separator
```

---

### Install Checkbox Component via CLI - Bash

Source: https://ui.shadcn.com/docs/components/checkbox

Command-line installation method for adding the checkbox component to a shadcn/ui project. Automatically handles component setup and dependency installation.

```bash
npx shadcn@latest add checkbox
```

---

### Install Aspect Ratio Dependencies Manually

Source: https://ui.shadcn.com/docs/components/aspect-ratio

Manually installs the required Radix UI aspect-ratio dependency. Use this approach when you prefer manual setup or when the CLI method is not suitable for your project.

```bash
npm install @radix-ui/react-aspect-ratio
```

---

### Install Input Component via CLI

Source: https://ui.shadcn.com/docs/components/input

Install the Input component using the shadcn CLI tool. This command downloads and sets up the component in your project's components directory with all necessary dependencies.

```bash
npx shadcn@latest add input
```

---

### Create Remix Project with create-remix

Source: https://ui.shadcn.com/docs/installation/remix

Initialize a new Remix project using the create-remix command-line tool. This sets up the basic Remix application structure and dependencies.

```bash
npx create-remix@latest my-app
```

---

### Install Shadcn UI Context Menu component via CLI (Bash)

Source: https://ui.shadcn.com/docs/components/context-menu

This command demonstrates how to easily add the Shadcn UI Context Menu component to your project using the `npx shadcn@latest add` command-line utility. This method automates the setup and configuration of the component.

```bash
npx shadcn@latest add context-menu
```

---

### Install Vaul Dependency for Manual Setup

Source: https://ui.shadcn.com/docs/components/drawer

Manually install the Vaul package as a dependency when setting up the Drawer component without the CLI. Vaul is the underlying library that powers the Drawer functionality.

```bash
npm install vaul
```

---

### Install Recharts Dependency via npm

Source: https://ui.shadcn.com/docs/components/chart

Installs the Recharts library as a project dependency for manual setup. Required when not using the CLI installation method.

```bash
npm install recharts
```

---

### Install Shadcn UI Command Component

Source: https://ui.shadcn.com/docs/components/command

This section provides instructions for installing the Command menu component, offering both an automated CLI approach and a manual method. The CLI command automatically adds the component, while the manual installation requires installing the 'cmdk' package and then copying the component source code separately.

```bash
npx shadcn@latest add command
```

```bash
npm install cmdk
```

---

### Install Components from Multiple Namespaced Registries

Source: https://ui.shadcn.com/docs/changelog

Use the @registry/name format to install components from different namespaced registries in a single command. Components are automatically resolved and installed from the correct registry sources.

```bash
npx shadcn add @acme/button @internal/auth-system
```

---

### Install Block and Override Primitives in shadcn/ui

Source: https://ui.shadcn.com/docs/registry/examples

Configure a registry item to install a block from shadcn/ui and override default primitives with custom implementations from remote registries. This enables centralized dependency management for component hierarchies.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "custom-login",
  "type": "registry:block",
  "registryDependencies": [
    "login-01",
    "https://example.com/r/button.json",
    "https://example.com/r/input.json",
    "https://example.com/r/label.json"
  ]
}
```

---

### Define Initial shadcn registry.json Structure

Source: https://ui.shadcn.com/docs/registry/getting-started

This JSON snippet illustrates the basic structure for a `registry.json` file, which serves as the entry point for a shadcn component registry. It includes the schema reference, registry name, homepage URL, and an empty array for registry items, conforming to the specified registry schema.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry.json",
  "name": "acme",
  "homepage": "https://acme.com",
  "items": [
    // ...
  ]
}
```

---

### List All Components from a Registry

Source: https://ui.shadcn.com/docs/changelog

Display all available components from a specified namespaced registry. Useful for discovering available components before installation.

```bash
npx shadcn list @acme
```

---

### Execute shadcn Registry Build Script

Source: https://ui.shadcn.com/docs/registry/getting-started

This command runs the `registry:build` script defined in `package.json`. Executing this script triggers the shadcn CLI to generate the registry JSON files, typically placed in a `public/r` directory by default.

```bash
npm run registry:build
```

---

### Install Shadcn UI Select Component via CLI

Source: https://ui.shadcn.com/docs/components/select

This command illustrates the quickest way to add the Shadcn UI Select component to your project. It utilizes the `npx shadcn@latest add` utility to automatically install dependencies and configure the component.

```bash
npx shadcn@latest add select
```

---

### Configure shadcn Build Script in package.json

Source: https://ui.shadcn.com/docs/registry/getting-started

This JSON snippet updates the `package.json` file by adding a `registry:build` script. This script executes the `shadcn build` command, which is used to generate the necessary JSON files for the component registry.

```json
{
  "scripts": {
    "registry:build": "shadcn build"
  }
}
```

---

### Install Resources from Namespaced Registries

Source: https://ui.shadcn.com/docs/components-json

Install components and resources using the namespace syntax after configuring registries. Supports installing from public registries, private authenticated registries, and multiple resources in a single command.

```bash
# Install from a configured registry
npx shadcn@latest add @v0/dashboard

# Install from private registry
npx shadcn@latest add @private/button

# Install multiple resources
npx shadcn@latest add @acme/header @internal/auth-utils
```

---

### Install Kbd Component via CLI (shadcn/ui)

Source: https://ui.shadcn.com/docs/components/kbd

Provides the command-line interface instruction to add the `Kbd` component to a project using `shadcn@latest`. This is the recommended and easiest method for integrating the component.

```bash
npx shadcn@latest add kbd
```

---

### Handle `shadcn/ui` Initialization with React 19 Peer Dependency Prompt (npm)

Source: https://ui.shadcn.com/docs/react-19

This `bash` snippet illustrates the interactive prompt from the `shadcn/ui` CLI when initializing a project (`npx shadcn@latest init -d`) while using React 19 with `npm`. It guides users to select a resolution strategy, either `--force` or `--legacy-peer-deps`, to address potential peer dependency conflicts during the shadcn/ui installation process.

```bash
It looks like you are using React 19.
Some packages may fail to install due to peer dependency issues (see https://ui.shadcn.com/react-19).

? How would you like to proceed? › - Use arrow-keys. Return to submit.
❯ Use --force
 Use --legacy-peer-deps
```

---

### Install shadcn/ui Label Component via CLI

Source: https://ui.shadcn.com/docs/components/label

This `bash` command uses the `shadcn/ui` CLI to quickly add the `Label` component to your project. It automates the process of fetching and integrating the component's files and dependencies, streamlining setup.

```bash
npx shadcn@latest add label
```

---

### Add Components to Monorepo Workspace

Source: https://ui.shadcn.com/docs/monorepo

Add shadcn/ui components to your monorepo application by navigating to the app directory and running the add command. The CLI automatically determines component type and installs files to correct paths with proper import handling.

```bash
cd apps/web
npx shadcn@latest add [COMPONENT]
```

---

### Install Shadcn UI Spinner Component via CLI (Bash)

Source: https://ui.shadcn.com/docs/components/spinner

Provides the command-line interface (CLI) instruction to add the Shadcn UI Spinner component to your project. This command automates the setup, including creating the component file and configuring necessary dependencies. Ensure you have the Shadcn UI CLI installed globally or locally before running this command.

```bash
npx shadcn@latest add spinner
```

---

### Install Drawer Component via CLI

Source: https://ui.shadcn.com/docs/components/drawer

Install the shadcn Drawer component using the CLI tool. This is the recommended installation method that automatically sets up all dependencies and copies necessary files to your project.

```bash
npx shadcn@latest add drawer
```

---

### Install Navigation Menu via CLI - shadcn/ui

Source: https://ui.shadcn.com/docs/components/navigation-menu

Quick installation command for adding the navigation-menu component to a shadcn/ui project using the CLI tool. Requires Node.js and npm to be installed.

```bash
npx shadcn@latest add navigation-menu
```

---

### View Registry Component Before Installation

Source: https://ui.shadcn.com/docs/changelog

Preview a component from a namespaced registry without installing it. Displays component code and all dependencies upfront for review.

```bash
npx shadcn view @acme/auth-system
```

---

### Install Shadcn Hover Card Component via CLI

Source: https://ui.shadcn.com/docs/components/hover-card

This command-line interface (CLI) snippet demonstrates how to add the Shadcn UI Hover Card component to your project using `npx shadcn@latest add`. This method automates the installation and setup process for the component, including copying necessary files and updating configurations.

```bash
npx shadcn@latest add hover-card
```

---

### Install Toggle Group Dependencies via npm

Source: https://ui.shadcn.com/docs/components/toggle-group

Install the required Radix UI toggle group dependency manually using npm. Required for projects that prefer manual component setup.

```bash
npm install @radix-ui/react-toggle-group
```

---

### Import and Use Button Component in TanStack Start

Source: https://ui.shadcn.com/docs/installation/tanstack

Import the Button component from the components/ui directory and render it in your application. This example shows basic usage within a React functional component in the app/routes/index.tsx file.

```tsx
import { Button } from "@/components/ui/button"

function App() {
 return (


Click me

 )
}
```

---

### Install Carousel Component via CLI

Source: https://ui.shadcn.com/docs/components/carousel

shadcn/ui CLI command to automatically install and configure the carousel component with all dependencies and file setup. Simplest method for adding the carousel to your project.

```bash
npx shadcn@latest add carousel
```

---

### Install Resizable Component using Shadcn CLI

Source: https://ui.shadcn.com/docs/components/resizable

This command-line interface (CLI) snippet shows how to add the `resizable` component to a project using the `shadcn` utility. It simplifies the installation process by automatically configuring the component and its dependencies.

```bash
npx shadcn@latest add resizable
```

---

### Install Menubar via CLI - Bash

Source: https://ui.shadcn.com/docs/components/menubar

Command to install the menubar component using the shadcn package manager CLI. This is the quickest installation method that automatically downloads and configures the component for your project.

```bash
npx shadcn@latest add menubar
```

---

### Install Radio Group via CLI - Bash

Source: https://ui.shadcn.com/docs/components/radio-group

Command-line interface installation method for adding the radio-group component to a shadcn/ui project. This is the recommended approach as it automatically handles file copying and setup.

```bash
npx shadcn@latest add radio-group
```

---

### Define reusable registry block with components

Source: https://ui.shadcn.com/docs/registry/examples

Create a registry block item that bundles multiple related files (pages and components) with their dependencies. This block specifies registry dependencies on other components and defines file paths with content references for installation into target locations in the project structure.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "login-01",
  "type": "registry:block",
  "description": "A simple login form.",
  "registryDependencies": ["button", "card", "input", "label"],
  "files": [
    {
      "path": "blocks/login-01/page.tsx",
      "content": "import { LoginForm } ...",
      "type": "registry:page",
      "target": "app/login/page.tsx"
    },
    {
      "path": "blocks/login-01/components/login-form.tsx",
      "content": "...",
      "type": "registry:component"
    }
  ]
}
```

---

### Install Radix UI Context Menu dependency manually (Bash)

Source: https://ui.shadcn.com/docs/components/context-menu

This command is part of the manual installation process, showing how to install the core `@radix-ui/react-context-menu` dependency using npm. This dependency provides the fundamental building blocks for the Shadcn UI Context Menu.

```bash
npm install @radix-ui/react-context-menu
```

---

### Install Native Select component via CLI

Source: https://ui.shadcn.com/docs/components/native-select

Use the shadcn CLI to easily add the Native Select component to your project. This command will scaffold the necessary files and update dependencies automatically, streamlining the setup process.

```bash
npx shadcn@latest add native-select
```

---

### Install Empty Component via CLI

Source: https://ui.shadcn.com/docs/components/empty

Command to install the Empty component using the shadcn package manager. Automatically adds the component and its dependencies to the project.

```bash
npx shadcn@latest add empty
```

---

### Install shadcn Table component and TanStack React Table

Source: https://ui.shadcn.com/docs/components/data-table

Installation commands to add the Table component from shadcn and the TanStack React Table dependency to your project. These are prerequisites for building data tables with this guide.

```bash
npx shadcn@latest add table
```

```bash
npm install @tanstack/react-table
```

---

### Install Switch Component via CLI

Source: https://ui.shadcn.com/docs/components/switch

Command-line installation method for adding the Switch component to a shadcn/ui project. Uses the official CLI tool to automatically download and configure the component with all required dependencies.

```bash
npx shadcn@latest add switch
```

---

### Start Development Server with pnpm

Source: https://ui.shadcn.com/docs/blocks

Starts the development server for the www application at http://localhost:3333. Enables live preview of blocks during development.

```bash
pnpm www:dev
```

---

### Install Shadcn UI Badge component via CLI (Bash)

Source: https://ui.shadcn.com/docs/components/badge

This command line interface snippet demonstrates how to add the Shadcn UI Badge component to a project using the `npx shadcn` utility. It simplifies the setup process by automating the component file generation.

```bash
npx shadcn@latest add badge
```

---

### Interactive Configuration Questions for shadcn init

Source: https://ui.shadcn.com/docs/changelog

Configuration prompts displayed during the shadcn init setup process. Users answer questions about style, base color, CSS file location, CSS variables usage, Tailwind config path, component/utils import aliases, and React Server Components support.

```text
Which style would you like to use? › Default
Which color would you like to use as base color? › Slate
Where is your global CSS file? › › app/globals.css
Do you want to use CSS variables for colors? › no / yes
Where is your tailwind.config.js located? › tailwind.config.js
Configure the import alias for components: › @/components
Configure the import alias for utils: › @/lib/utils
Are you using React Server Components? › no / yes
```

---

### Item Component Installation - Bash

Source: https://ui.shadcn.com/docs/components/item

CLI command to install the Item component from shadcn. Requires Node.js and npm/pnpm package manager.

```bash
npx shadcn@latest add item
```

---

### CLI Command: Initialize Project from Local File

Source: https://ui.shadcn.com/docs/changelog

The `shadcn` CLI now supports initializing projects from local JSON files. This command allows users to set up a project using a local `template.json`, enabling zero-setup development and local testing of registry items.

```bash
npx shadcn init ./template.json
```

---

### Install Tailwind CSS and Autoprefixer

Source: https://ui.shadcn.com/docs/installation/remix

Install Tailwind CSS and Autoprefixer as development dependencies to enable styling support for shadcn/ui components in your Remix project.

```bash
npm install -D tailwindcss@latest autoprefixer@latest
```

---

### Install Tooltip Dependencies via npm

Source: https://ui.shadcn.com/docs/components/tooltip

Manual installation of the Radix UI tooltip dependency. Required when not using the shadcn CLI installation method. Install this package before copying the tooltip component source.

```bash
npm install @radix-ui/react-tooltip
```

---

### Install Shadcn UI Dialog component using CLI or npm

Source: https://ui.shadcn.com/docs/components/dialog

Instructions for installing the Shadcn UI Dialog component. Provides options for using the Shadcn CLI to add the component or manually installing the underlying Radix UI dependency.

```bash
npx shadcn@latest add dialog
```

```bash
npm install @radix-ui/react-dialog
```

---

### Install Toggle Component via CLI

Source: https://ui.shadcn.com/docs/components/toggle

Install the Toggle component using the shadcn CLI tool. This command downloads and sets up the component with all dependencies in your project.

```bash
npx shadcn@latest add toggle
```

---

### Install Sheet Component via CLI

Source: https://ui.shadcn.com/docs/components/sheet

Command to install the Sheet component and its dependencies using the shadcn CLI. This is the recommended installation method for projects using shadcn/ui.

```bash
npx shadcn@latest add sheet
```

---

### Install Shadcn UI Popover component

Source: https://ui.shadcn.com/docs/components/popover

These commands provide two methods for installing the Popover component into your project. The CLI method uses `npx shadcn` to add the component automatically, while the manual method involves installing the core Radix UI dependency via npm and then copying the component source code. Ensure your project is set up to use shadcn/ui before installing components.

```bash
npx shadcn@latest add popover
```

```bash
npm install @radix-ui/react-popover
```

---

### Install Sonner via CLI

Source: https://ui.shadcn.com/docs/components/sonner

Command-line installation method using shadcn-cli to add the Sonner component to a project. This is the quickest way to set up Sonner with all necessary dependencies.

```bash
npx shadcn@latest add sonner
```

---

### Complete Bar Chart with XAxis Implementation

Source: https://ui.shadcn.com/docs/components/chart

Full React component example using the 'use client' directive for client-side rendering. Demonstrates a complete bar chart setup with sample data for desktop and mobile metrics across six months, including XAxis configuration with custom tick formatting.

```tsx
"use client"

import { Bar, BarChart, CartesianGrid, XAxis } from "recharts"

import { ChartConfig, ChartContainer } from "@/components/ui/chart"

const chartData = [
 { month: "January", desktop: 186, mobile: 80 },
 { month: "February", desktop: 305, mobile: 200 },
 { month: "March", desktop: 237, mobile: 120 },
 { month: "April", desktop: 73, mobile: 190 },
 { month: "May", desktop: 209, mobile: 130 },
 { month: "June", desktop: 214, mobile: 140 },
]

const chartConfig = {
 desktop: {
 label: "Desktop",
 color: "#2563eb",
 },
 mobile: {
 label: "Mobile",
 color: "#60a5fa",
 },
} satisfies ChartConfig

export function Component() {
 return (
  value.slice(0, 3)}
 />

 )
}
```

---

### Environment Variables Setup

Source: https://ui.shadcn.com/docs/registry/authentication

Set registry authentication token in .env.local file. This stores the secret token that will be used for Bearer authentication when accessing private component registries.

```bash
REGISTRY_TOKEN=your_secret_token_here
```

---

### Install Table Component via CLI

Source: https://ui.shadcn.com/docs/components/table

CLI command to install the shadcn/ui Table component using npx. This automatically adds the component to your project.

```bash
npx shadcn@latest add table
```

---

### Install next-themes package

Source: https://ui.shadcn.com/docs/dark-mode/next

This command installs the `next-themes` package, a crucial dependency for implementing dark mode functionality in Next.js applications.

```bash
npm install next-themes
```

---

### Install Separator Component via CLI

Source: https://ui.shadcn.com/docs/components/separator

Install the Separator component using the shadcn CLI tool. This command automatically downloads and sets up the component in your project with all required dependencies.

```bash
npx shadcn@latest add separator
```

---

### Install Menubar Dependencies - Bash

Source: https://ui.shadcn.com/docs/components/menubar

Manual installation command for the Radix UI menubar dependency. Use this when manually setting up the component instead of using the CLI. Requires Node.js package manager (npm).

```bash
npm install @radix-ui/react-menubar
```

---

### Install Tooltip via shadcn CLI

Source: https://ui.shadcn.com/docs/components/tooltip

Command-line installation method for adding the Tooltip component to a shadcn/ui project. This is the recommended approach for quickly adding pre-configured component files.

```bash
npx shadcn@latest add tooltip
```

---

### Install Slider Component via CLI

Source: https://ui.shadcn.com/docs/components/slider

Command-line installation method for adding the Slider component to a shadcn/ui project. This is the quickest way to install the component and its dependencies.

```bash
npx shadcn@latest add slider
```

---

### Install Shadcn Alert component via CLI

Source: https://ui.shadcn.com/docs/components/alert

This command provides a quick way to add the Shadcn Alert component to your project using the command-line interface. It leverages `npx` to execute the `shadcn` utility for component installation.

```bash
npx shadcn@latest add alert
```

---

### Create a Basic shadcn Component in TSX

Source: https://ui.shadcn.com/docs/registry/getting-started

This TypeScript React (TSX) code defines a simple `HelloWorld` component that renders a button with 'Hello World' text. It imports the `Button` component from a local UI library, demonstrating how to structure a component intended for the shadcn registry.

```tsx
import { Button } from "@/components/ui/button"

export function HelloWorld() {
 return Hello World
}
```

---

### Install Radix UI Switch Dependency

Source: https://ui.shadcn.com/docs/components/switch

NPM installation command for the Radix UI switch primitive dependency. Required when manually installing the Switch component without using the shadcn CLI tool.

```bash
npm install @radix-ui/react-switch
```

---

### Button Size Variants Example

Source: https://ui.shadcn.com/docs/components/button

Comprehensive example showing all Button size options: sm, icon-sm, default, icon, lg, and icon-lg. Demonstrates text and icon buttons at different sizes.

```typescript
import { ArrowUpRightIcon } from "lucide-react"

import { Button } from "@/components/ui/button"

export function ButtonSize() {
 return (


Small


Default

Large


 )
}
```

---

### Create custom style extending shadcn/ui

Source: https://ui.shadcn.com/docs/registry/examples

Define a custom registry style that extends shadcn/ui by installing dependencies, adding registry dependencies (components and remote blocks), and configuring CSS variables for fonts and brand colors in light and dark modes. This configuration is applied when running `npx shadcn init`.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "example-style",
  "type": "registry:style",
  "dependencies": ["@tabler/icons-react"],
  "registryDependencies": ["login-01", "calendar", "https://example.com/r/editor.json"],
  "cssVars": {
    "theme": {
      "font-sans": "Inter, sans-serif"
    },
    "light": {
      "brand": "20 14.3% 4.1%"
    },
    "dark": {
      "brand": "20 14.3% 4.1%"
    }
  }
}
```

---

### Example Shadcn UI Registry Configuration (JSON)

Source: https://ui.shadcn.com/docs/registry/registry-index

This JSON configuration demonstrates a valid structure for a Shadcn UI registry. It includes a schema reference, the registry's name and homepage, and an array of items, each representing a component or example with its type, title, description, and associated file paths. This structure adheres to the specified registry schema requirements.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry.json",
  "name": "acme",
  "homepage": "https://acme.com",
  "items": [
    {
      "name": "login-form",
      "type": "registry:component",
      "title": "Login Form",
      "description": "A login form component.",
      "files": [
        {
          "path": "registry/new-york/auth/login-form.tsx",
          "type": "registry:component"
        }
      ]
    },
    {
      "name": "example-login-form",
      "type": "registry:component",
      "title": "Example Login Form",
      "description": "An example showing how to use the login form component.",
      "files": [
        {
          "path": "registry/new-york/examples/example-login-form.tsx",
          "type": "registry:component"
        }
      ]
    }
  ]
}
```

---

### Manually install Radix UI Alert Dialog dependency with npm

Source: https://ui.shadcn.com/docs/components/alert-dialog

This `bash` command is used for manual installation of the Shadcn UI Alert Dialog component, specifically for installing its underlying Radix UI primitive. It adds the `@radix-ui/react-alert-dialog` package as a dependency to your project. This step is required before copying the component source code.

```bash
npm install @radix-ui/react-alert-dialog
```

---

### Component Diff Output Example

Source: https://ui.shadcn.com/docs/changelog

Example output showing differences in a component's code. The diff displays additions and removals, showing what has changed in the upstream repository.

```diff
const alertVariants = cva(
- "relative w-full rounded-lg border",
+ "relative w-full pl-12 rounded-lg border"
)
```

---

### Install Button Dependencies via npm

Source: https://ui.shadcn.com/docs/components/button

Manual installation of required dependencies for the Button component. Install the @radix-ui/react-slot package which provides slot composition functionality.

```bash
npm install @radix-ui/react-slot
```

---

### CLI Error: Missing Registry Environment Variables

Source: https://ui.shadcn.com/docs/changelog

This example demonstrates the CLI's helpful error for missing environment variables required by a registry. It explicitly lists the necessary variables, like `REGISTRY_TOKEN`, and instructs users to set them in `.env` or `.env.local` files.

```txt
Registry "@private" requires the following environment variables:
 • REGISTRY_TOKEN

Set the required environment variables to your .env or .env.local file.
```

---

### Install Multiple Resources from Different Namespaces

Source: https://ui.shadcn.com/docs/registry/namespace

Install multiple resources from different namespaced registries in a single command. Supports combining resources from UI components, libraries, and AI prompts across various registries.

```bash
npx shadcn@latest add @acme/header @lib/auth-utils @ai/chatbot-rules
```

---

### Define Universal Registry Item for ESLint Configuration (shadcn/ui)

Source: https://ui.shadcn.com/docs/registry/examples

This JSON configuration defines a shadcn/ui registry item named 'my-eslint-config' for a custom ESLint configuration. It specifies a single file with an explicit target path (~/.eslintrc.json), enabling universal installation of the ESLint config file without framework dependencies.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "my-eslint-config",
  "type": "registry:item",
  "files": [
    {
      "path": "/path/to/your/registry/default/custom-eslint.json",
      "type": "registry:file",
      "target": "~/.eslintrc.json",
      "content": "..."
    }
  ]
}
```

---

### Configure Plugin with NPM Dependencies in shadcn UI

Source: https://ui.shadcn.com/docs/registry/examples

Shows how to include external npm packages as dependencies when using Tailwind CSS plugins. The `dependencies` array declares required packages, while the `css` object configures both the plugin and custom CSS layers. This pattern ensures all required packages are installed before the component is used.

```json
{
 "$schema": "https://ui.shadcn.com/schema/registry-item.json",
 "name": "typography-component",
 "type": "registry:item",
 "dependencies": ["@tailwindcss/typography"],
 "css": {
 "@plugin "@tailwindcss/typography"": {},
 "@layer components": {
 ".prose": {
 "max-width": "65ch"
 }
 }
 }
}
```

---

### Install Shadcn Accordion Component (bash)

Source: https://ui.shadcn.com/docs/components/accordion

This snippet provides two methods for installing the Shadcn UI Accordion component. Users can either add the component directly using the Shadcn CLI or manually install the underlying Radix UI dependency via npm. Both methods prepare the project for using the Accordion component by adding necessary files and packages.

```bash
npx shadcn@latest add accordion
```

```bash
npm install @radix-ui/react-accordion
```

---

### Install Navigation Menu Dependencies - npm

Source: https://ui.shadcn.com/docs/components/navigation-menu

Manual installation of required Radix UI navigation menu dependency. Use this approach when manually setting up the component instead of using the CLI.

```bash
npm install @radix-ui/react-navigation-menu
```

---

### Configure Secure Custom Registry with Authorization Headers (JSON)

Source: https://ui.shadcn.com/docs/registry/namespace

Provides an example of configuring a custom company registry in `components.json`, including a URL and authorization headers with an environment variable. This setup demonstrates best practices for securely connecting to private registries, requiring explicit authentication.

```json
{
  "@company": {
    "url": "https://registry.company.com/v1/{name}.json",
    "headers": {
      "Authorization": "Bearer ${COMPANY_TOKEN}",
      "X-Registry-Version": "1.0"
    }
  }
}
```

---

### Create components.json Configuration File for shadcn/ui

Source: https://ui.shadcn.com/docs/installation/manual

This JSON configuration file sets up the shadcn/ui component library with New York style, TypeScript/TSX support, Tailwind CSS styling with CSS variables, and path aliases for easier imports. Place this file in the root of your project directory to enable component scaffolding and configuration.

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "new-york",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/styles/globals.css",
    "baseColor": "neutral",
    "cssVariables": true,
    "prefix": ""
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  },
  "iconLibrary": "lucide"
}
```

---

### Install Button Component via CLI

Source: https://ui.shadcn.com/docs/components/button

Quick installation of the Button component using the shadcn CLI tool. Automatically adds the button component and its dependencies to your project.

```bash
npx shadcn@latest add button
```

---

### Install Tabs Dependencies via NPM - Bash

Source: https://ui.shadcn.com/docs/components/tabs

Manual npm installation of the Radix UI Tabs dependency. Use this method when manually adding the tabs component instead of using the CLI installer.

```bash
npm install @radix-ui/react-tabs
```

---

### Install Checkbox Dependencies - Bash

Source: https://ui.shadcn.com/docs/components/checkbox

Manual installation command for the Radix UI checkbox dependency. Required when manually setting up the checkbox component without using the CLI.

```bash
npm install @radix-ui/react-checkbox
```

---

### Add Components with add Command

Source: https://ui.shadcn.com/docs/cli

The add command installs specific components and their dependencies into your project. It supports single or multiple component installation, file overwriting, and path customization.

```bash
npx shadcn@latest add [component]
```

```bash
Usage: shadcn add [options] [components...]

add a component to your project

Arguments:
 components name, url or local path to component

Options:
 -y, --yes skip confirmation prompt. (default: false)
 -o, --overwrite overwrite existing files. (default: false)
 -c, --cwd  the working directory. defaults to the current directory.
 -a, --all add all available components (default: false)
 -p, --path  the path to add the component to.
 -s, --silent mute output. (default: false)
 --src-dir use the src directory when creating a new project. (default: false)
 --no-src-dir do not use the src directory when creating a new project.
 --css-variables use css variables for theming. (default: true)
 --no-css-variables do not use css variables for theming.
 -h, --help display help for command
```

---

### Create new React project with Vite

Source: https://ui.shadcn.com/docs/installation/vite

Initializes a new React project using Vite. This command uses the latest version of Vite and allows selecting the 'React + TypeScript' template during the interactive setup process.

```bash
npm create vite@latest
```

---

### Manually Install Radix UI Label npm Dependency

Source: https://ui.shadcn.com/docs/components/label

For manual installation of the `shadcn/ui` Label component, this `npm` command installs its core dependency, `@radix-ui/react-label`. This step is followed by copying the component's source code into your project and updating import paths.

```bash
npm install @radix-ui/react-label
```

---

### Install Single Resource from Namespaced Registry

Source: https://ui.shadcn.com/docs/registry/namespace

Install a single resource from a configured namespace using the shadcn CLI. The syntax uses @namespace/resource-name format to specify which registry and resource to install.

```bash
npx shadcn@latest add @v0/dashboard
```

---

### Add UI Components with add Command

Source: https://ui.shadcn.com/docs/changelog

Use the add command to install UI components from shadcn into your project. The CLI automatically resolves dependencies, formats components based on your components.json configuration, and installs them with correct import paths and styling methods.

```bash
npx shadcn@latest add
```

---

### Install Button Group via CLI - Bash

Source: https://ui.shadcn.com/docs/components/button-group

Command-line installation script for the Button Group component using the shadcn package manager. This is the recommended installation method that automatically sets up the component with dependencies.

```bash
npx shadcn@latest add button-group
```

---

### Radix UI Migration: Import Path Update Example

Source: https://ui.shadcn.com/docs/changelog

This `diff` example illustrates the effect of the `radix` migration command on component files. It shows how an import for `AlertDialogPrimitive` is changed from `@radix-ui/react-dialog` to the new `radix-ui` package.

```diff
- import * as AlertDialogPrimitive from "@radix-ui/react-dialog"
+ import { AlertDialog as AlertDialogPrimitive } from "radix-ui"
```

---

### Initialize shadcn Project with init Command

Source: https://ui.shadcn.com/docs/cli

The init command sets up a new shadcn project by installing dependencies, adding the cn utility, and configuring CSS variables. It supports template selection, base color configuration, and directory structure options.

```bash
npx shadcn@latest init
```

```bash
Usage: shadcn init [options] [components...]

initialize your project and install dependencies

Arguments:
 components name, url or local path to component

Options:
 -t, --template  the template to use. (next, next-monorepo)
 -b, --base-color  the base color to use. (neutral, gray, zinc, stone, slate)
 -y, --yes skip confirmation prompt. (default: true)
 -f, --force force overwrite of existing configuration. (default: false)
 -c, --cwd  the working directory. defaults to the current directory.
 -s, --silent mute output. (default: false)
 --src-dir use the src directory when creating a new project. (default: false)
 --no-src-dir do not use the src directory when creating a new project.
 --css-variables use css variables for theming. (default: true)
 --no-css-variables do not use css variables for theming.
 --no-base-style do not install the base shadcn style
 -h, --help display help for command
```

---

### Install Dropdown Menu Dependencies

Source: https://ui.shadcn.com/docs/components/dropdown-menu

NPM installation command for the Radix UI dropdown menu primitive dependency. Required when manually adding the component without using the shadcn/ui CLI tool.

```bash
npm install @radix-ui/react-dropdown-menu
```

---

### Install Toggle Component Dependencies Manually

Source: https://ui.shadcn.com/docs/components/toggle

Install the required Radix UI toggle dependency manually for projects that don't use the shadcn CLI. This is the first step when manually setting up the Toggle component.

```bash
npm install @radix-ui/react-toggle
```

---

### Add Components with Shadcn CLI

Source: https://ui.shadcn.com/docs/changelog

This command demonstrates how to use the Shadcn CLI to add a specific component from a registry to your project. It automatically handles installation and updates the project's 'components.json' file.

```bash
npx shadcn add @ai-elements/prompt-input
```

---

### Initialize MCP Server for shadcn Registries

Source: https://ui.shadcn.com/docs/changelog

Set up MCP (Model Context Protocol) server for all configured registries with zero configuration. Enables integration with MCP clients for AI-assisted component discovery and installation.

```bash
npx shadcn@latest mcp init
```

---

### Install Chart Component via CLI

Source: https://ui.shadcn.com/docs/components/chart

Installs the chart.tsx component using shadcn's CLI tool. This command automatically sets up the chart component file in the project structure.

```bash
npx shadcn@latest add chart
```

---

### Install Card Component via CLI - shadcn

Source: https://ui.shadcn.com/docs/components/card

Install the Card component using the shadcn CLI tool. This command downloads and integrates the Card component into your project's components directory.

```bash
npx shadcn@latest add card
```
