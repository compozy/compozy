### Add All shadcn/ui Components using CLI

Source: https://ui.shadcn.com/docs/installation/tanstack

This command installs all available shadcn/ui components into your TanStack Start project using the shadcn/ui CLI. It requires a package manager like pnpm, npm, yarn, or bun.

```shell
pnpm dlx shadcn@latest add --all
```

---

### Create TanStack Start Project with shadcn/ui

Source: https://ui.shadcn.com/docs/installation/tanstack

This command initializes a new TanStack Start project and integrates shadcn/ui, including Tailwind CSS. It requires pnpm, npm, yarn, or bun to run.

```shell
pnpm create @tanstack/start@latest --tailwind --add-ons shadcn
```

---

### Custom Block Installation

Source: https://ui.shadcn.com/docs/registry/examples

Specifies a custom block to be installed from the shadcn/ui registry. In this example, it installs the 'login-01' block.

```json
{
  "$schema": "https://u"
}
```

---

### Example: Multiple Registry Setup in components.json

Source: https://ui.shadcn.com/docs/components-json

Demonstrates a complex `components.json` configuration with multiple registries, including public, private with authentication, and team-specific resources. This setup allows for flexible installation of components and utilities from diverse sources.

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

### Universal Item with Multiple Files (JSON)

Source: https://ui.shadcn.com/docs/registry/examples

An example of a universal registry item that installs multiple files, including dependencies. This is useful for starter templates or comprehensive configurations, with explicit `target` paths for each file.

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

### Create Gatsby Project using npm

Source: https://ui.shadcn.com/docs/installation/gatsby

Initializes a new Gatsby project using the npm command. This command is the starting point for setting up a new Gatsby application.

```bash
npm init gatsby
```

---

### Create React Project with Vite (TypeScript)

Source: https://ui.shadcn.com/docs/installation/vite

Command to create a new React project using Vite with the TypeScript template. This is the initial step for setting up the project.

```bash
pnpm create vite@latest
 --template react-ts
```

---

### Install Component from Private Registry

Source: https://ui.shadcn.com/docs/components-json

Command-line instruction to install a component from a private registry using its alias. This example, `@private/button`, demonstrates fetching resources from a protected registry defined in `components.json`.

```bash
npx shadcn@latest add @private/button
```

---

### Install Component from Configured Registry

Source: https://ui.shadcn.com/docs/components-json

Command-line instruction to install a component using a configured registry alias. The `@v0/dashboard` syntax specifies the registry and the resource name, leveraging the `components.json` setup.

```bash
npx shadcn@latest add @v0/dashboard
```

---

### Install Tailwind CSS

Source: https://ui.shadcn.com/docs/installation/remix

Commands to install Tailwind CSS and Autoprefixer as development dependencies in a Remix project using pnpm, npm, or yarn.

```bash
pnpm add -D tailwindcss@latest autoprefixer@latest
```

---

### Universal Item for ESLint Configuration (JSON)

Source: https://ui.shadcn.com/docs/registry/examples

This example shows a universal registry item for installing a custom ESLint configuration. It uses an explicit `target` to place the `.eslintrc.json` file in the user's home directory, ensuring framework independence.

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

### Install Switch Component (Manual)

Source: https://ui.shadcn.com/docs/components/switch

Manual installation instructions for the Switch component, providing alternative methods using pnpm, npm, or yarn. This approach is useful if the CLI method is not preferred or feasible.

```bash
pnpm add switch
npm install switch
yarn add switch
```

---

### Installing shadcn/ui Spinner Component

Source: https://ui.shadcn.com/docs/components/spinner

Shows how to add the Spinner component to your project using the shadcn-ui CLI. This command-line installation method simplifies dependency management and setup.

```bash
pnpm dlx shadcn@latest add spinner
```

---

### Install Switch Component (CLI)

Source: https://ui.shadcn.com/docs/components/switch

Instructions for installing the Switch component using the shadcn-ui CLI. This is the recommended method for adding components to your project. Ensure you have the shadcn-ui CLI installed and configured.

```bash
pnpm dlx shadcn@latest add switch
```

---

### shadcn/ui Select Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/select

Instructions for installing the shadcn/ui Select component using the pnpm, npm, or yarn package managers. This command-line interface method simplifies the process of adding the component to your project. Ensure you have the shadcn/ui CLI installed globally.

```bash
pnpm dlx shadcn@latest add select
npx shadcn@latest add select
yarn dlx shadcn@latest add select
```

---

### Installing Components from Namespaced Registries

Source: https://ui.shadcn.com/docs/changelog

Demonstrates how to install components using the shadcn CLI from different registered namespaces. This enables decentralized component distribution and management.

```shell
pnpm dlx shadcn add @acme/button @internal/auth-system
```

---

### Custom Style from Scratch

Source: https://ui.shadcn.com/docs/registry/examples

Creates a custom style from scratch without extending shadcn/ui. It installs specific npm packages and registry items, and defines new CSS variables for theme colors.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "extends": "none",
  "name": "new-style",
  "type": "registry:style",
  "dependencies": ["tailwind-merge", "clsx"],
  "registryDependencies": [
    "utils",
    "https://example.com/r/button.json",
    "https://example.com/r/input.json",
    "https://example.com/r/label.json",
    "https://example.com/r/select.json"
  ],
  "cssVars": {
    "theme": {
      "font-sans": "Inter, sans-serif"
    },
    "light": {
      "main": "#88aaee",
      "bg": "#dfe5f2",
      "border": "#000",
      "text": "#000",
      "ring": "#000"
    },
    "dark": {
      "main": "#88aaee",
      "bg": "#272933",
      "border": "#000",
      "text": "#e6e6e6",
      "ring": "#fff"
    }
  }
}
```

---

### Import and Use Button Component in React

Source: https://ui.shadcn.com/docs/installation/vite

Example of importing the 'Button' component from shadcn/ui and using it within a React component. Assumes the Button component has been added via the CLI.

```typescript
import { Button } from "@/components/ui/button"

function App() {
 return (
 Click me
 )
}

export default App
```

---

### Add Dependencies to Project (npm, pnpm, yarn, bun)

Source: https://ui.shadcn.com/docs/installation/manual

Installs the required dependencies for shadcn/ui using package managers. Ensure Tailwind CSS is already set up in your project.

```bash
pnpm add class-variance-authority clsx tailwind-merge lucide-react tw-animate-css
```

---

### shadcn/ui Tooltip Installation (CLI)

Source: https://ui.shadcn.com/docs/components/tooltip

This command installs the shadcn/ui Tooltip component using the shadcn CLI. This is the recommended method for adding components to your project, ensuring proper dependency management.

```bash
pnpm dlx shadcn@latest add tooltip
```

---

### Install Multiple Resources from Different Registries

Source: https://ui.shadcn.com/docs/components-json

Command-line instruction to install multiple resources from various configured registries simultaneously. This showcases the flexibility of installing components like `@acme/header` and `@internal/auth-utils` in one go.

```bash
npx shadcn@latest add @acme/header @internal/auth-utils
```

---

### shadcn/ui Alert Component Import Example

Source: https://ui.shadcn.com/docs/components/alert

Example of how to import the Alert, AlertDescription, and AlertTitle components from shadcn/ui after installation. This allows you to use them in your React components.

```jsx
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
```

---

### Initialize shadcn/ui Project with CLI

Source: https://ui.shadcn.com/docs/cli

Initializes your project with shadcn/ui, installing necessary dependencies and configuring the project. This command sets up the `cn` utility and CSS variables.

```bash
pnpm dlx shadcn@latest init
```

---

### shadcn/ui CLI Installation Commands

Source: https://ui.shadcn.com/docs/cli

Shows the commands for installing shadcn/ui dependencies using different package managers (PNPM, NPM, Yarn, Bun).

```bash
pnpm dlx shadcn@latest init
```

```bash
npm dlx shadcn@latest init
```

```bash
yarn dlx shadcn@latest init
```

```bash
bun dlx shadcn@latest init
```

---

### shadcn/ui Empty Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/empty

Shows how to install the Empty component using the shadcn/ui CLI. This involves running a specific command to add the component to your project.

```bash
pnpm dlx shadcn@latest add empty
```

---

### Add Tailwind CSS and Vite Plugin

Source: https://ui.shadcn.com/docs/installation/vite

Installs Tailwind CSS and its Vite plugin. These are necessary for styling components with Tailwind utility classes.

```bash
pnpm add -D tailwindcss postcss autoprefixer
pnpm add -D @tailwindcss/vite
```

---

### Import and Use 'Button' Component in React

Source: https://ui.shadcn.com/docs/installation/tanstack

Demonstrates how to import and use the 'Button' component from shadcn/ui within a React component in a TanStack Start application. Assumes the 'Button' component has been added via the CLI.

```jsx
import { Button } from "@/components/ui/button"

function App() {
 return (
 Click me
 )
}
```

---

### shadcn/ui Alert Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/alert

Instructions for installing the Alert component using the shadcn/ui CLI. This command adds the necessary dependencies and component files to your project.

```bash
pnpm dlx shadcn@latest add alert
```

---

### Universal Item for Python Rules (JSON)

Source: https://ui.shadcn.com/docs/registry/examples

An example of a universal registry item designed to install custom Python rules for the Cursor editor. It specifies an explicit target path for the rule file, making it framework-agnostic.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "python-rules",
  "type": "registry:item",
  "files": [
    {
      "path": "/path/to/your/registry/default/custom-python.mdc",
      "type": "registry:file",
      "target": "~/.cursor/rules/custom-python.mdc",
      "content": "..."
    }
  ]
}
```

---

### Calendar Installation (CLI)

Source: https://ui.shadcn.com/docs/components/calendar

Provides commands for installing the Calendar component using different package managers like pnpm, npm, yarn, and bun. This is a prerequisite for using the component in your project.

```bash
pnpm dlx shadcn@latest add calendar
```

---

### Dropdown Menu Basic Usage (JavaScript/React)

Source: https://ui.shadcn.com/docs/components/dropdown-menu

This snippet shows the minimal setup for a dropdown menu component. It requires importing the necessary components from shadcn/ui and defining a trigger and content. This is a foundational example for integrating dropdown menus.

```javascript
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
```

```javascript
OpenMy Account
 Profile
 ⇧⌘P
 Billing
 ⌘B
 Settings
 ⌘S
 Keyboard shortcuts
 ⌘K
 Team

 Invite users

 New Team...
 ⌘+T
 GitHub

 Support

 API

 Log out
 ⇧⌘Q

```

---

### Install Progress Component using CLI

Source: https://ui.shadcn.com/docs/components/progress

Command to add the Progress component to your project using the shadcn-ui CLI. This is the recommended way to install components.

```bash
pnpm dlx shadcn@latest add progress
```

---

### Separator Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/separator

Shows the command to install the Separator component using the shadcn-ui CLI. This is the recommended method for adding components.

```bash
pnpm dlx shadcn@latest add separator
```

---

### Add CSS Import with URL Syntax

Source: https://ui.shadcn.com/docs/registry/examples

This example demonstrates importing CSS files using the `url()` syntax, including importing from external sources like Google Fonts and local files.

```json
{
 "$schema": "https://ui.shadcn.com/schema/registry-item.json",
 "name": "font-import",
 "type": "registry:item",
 "css": {
 "@import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap")": {},
 "@import url('./local-styles.css')": {}
 }
}
```

---

### React Card Component Example (shadcn/ui)

Source: https://ui.shadcn.com/docs/components/card

This code snippet demonstrates how to use the Card component from shadcn/ui in a React application. It includes examples for header, title, description, content, footer, and action elements. Ensure you have the 'card' component installed via the shadcn/ui CLI or manually imported.

```jsx
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle, } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export function CardDemo() {
 return (
 Login to your accountEnter your email below to login to your account

Name

Framework

CancelDeploy
 )
}
```

---

### Install shadcn/ui Toggle Component

Source: https://ui.shadcn.com/docs/components/toggle

Instructions for installing the shadcn/ui Toggle component using pnpm. This command adds the necessary package and dependencies to your project.

```bash
pnpm dlx shadcn@latest add toggle
```

---

### Create a Simple Component (TypeScript/React)

Source: https://ui.shadcn.com/docs/registry/getting-started

An example of a basic component file, `hello-world.tsx`, intended to be part of a registry. This component uses a Button from `@/components/ui/button`. It should be placed within a structured directory like `registry/[NAME]/[COMPONENT_NAME]/`.

```tsx
import { Button } from "@/components/ui/button"

export function HelloWorld() {
 return (
 Hello World
 )
}
```

---

### shadcn/ui Alert Dialog Installation

Source: https://ui.shadcn.com/docs/components/alert-dialog

Instructions for installing the Alert Dialog component using the shadcn-ui CLI. This command adds the necessary component files to your project, allowing you to import and use them.

```bash
pnpm dlx shadcn@latest add alert-dialog
```

---

### Install Remote Component using URL (CLI)

Source: https://ui.shadcn.com/docs/changelog

Installs a remote component by providing its registry URL to the shadcn CLI. This allows for easy integration of components from external sources. It requires the shadcn CLI to be installed.

```bash
npx shadcn add https://acme.com/registry/navbar.json
```

---

### Add Functional CSS Utilities

Source: https://ui.shadcn.com/docs/registry/examples

This example demonstrates functional CSS utilities using a wildcard. The 'tab-\*' utility allows for dynamic application of 'tab-size' based on the value provided.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "custom-component",
  "type": "registry:component",
  "css": {
    "@utility tab-*": {
      "tab-size": "var(--tab-size-)"
    }
  }
}
```

---

### Install shadcn/ui Registry Item via CLI

Source: https://ui.shadcn.com/docs/registry/getting-started

Installs a registry item using the shadcn CLI by providing the URL of the registry item. This command allows developers to easily add components from a remote registry to their project.

```Shell
pnpm dlx shadcn@latest add http://localhost:3000/r/hello-world.json
```

---

### shadcn/ui Drawer Component Usage Example

Source: https://ui.shadcn.com/docs/components/drawer

Provides a basic import statement for using the Drawer component and its associated sub-components from shadcn/ui. This snippet is intended for developers who have installed the component and need to integrate it into their React application.

```javascript
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
```

---

### Toggle Group Component Documentation

Source: https://ui.shadcn.com/docs/components/toggle-group

This section details the Toggle Group component, its installation, usage, and various examples demonstrating its functionality and appearance.

````APIDOC
## Toggle Group Component

### Description
A set of two-state buttons that can be toggled on or off. Provides "single" and "multiple" selection modes.

### Installation
Use the shadcn-ui CLI to add the component to your project:
```bash
pnpm dlx shadcn@latest add toggle-group
````

### Usage

Import the necessary components and use them in your React application.

```javascript
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

function MyToggleGroup() {
 return (

 A

 B

 C

 );
}
```

### Examples

#### Outline

```javascript
import { Bold, Italic, Underline } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupOutline() {
 return (

 );
}
```

#### Single

```javascript
import { Bold, Italic, Underline } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupSingle() {
 return (

 );
}
```

#### Small

```javascript
import { Bold, Italic, Underline } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupSmall() {
 return (

 );
}
```

#### Large

```javascript
import { Bold, Italic, Underline } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupLarge() {
 return (

 );
}
```

#### Disabled

```javascript
import { Bold, Italic, Underline } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupDisabled() {
 return (

 );
}
```

#### Spacing

Use `spacing={2}` to add spacing between toggle group items.

```javascript
import { BookmarkIcon, HeartIcon, StarIcon } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function ToggleGroupSpacing() {
 return (

 );
}
```

### API Reference

#### ToggleGroup

The main component that wraps toggle group items.

| Prop        | Type       | Default     |
| ----------- | ---------- | ----------- | ----------- | ----------- |
| `type`      | `"single"  | "multiple"` | `"single"`  |
| `variant`   | `"default" | "outline"`  | `"default"` |
| `size`      | `"default" | "sm"        | "lg"`       | `"default"` |
| `spacing`   | `number`   | `0`         |
| `className` | `string`   | `''`        |

#### ToggleGroupItem

Individual toggle items within a toggle group. Remember to add an `aria-label` to each item for accessibility.

| Prop        | Type     | Default  |
| ----------- | -------- | -------- |
| `value`     | `string` | Required |
| `className` | `string` | `''`     |

````

--------------------------------

### Use shadcn/ui Button Component in Remix

Source: https://ui.shadcn.com/docs/installation/remix

Example of importing and using the shadcn/ui Button component within a Remix route component. Demonstrates basic component integration.

```typescript
import { Button } from "~/components/ui/button"

export default function Home() {
 return (
 Click me
 )
}
````

---

### Override Primitives with a Block

Source: https://ui.shadcn.com/docs/registry/examples

Demonstrates how to define a custom login block that overrides existing primitives like 'button', 'input', and 'label' with remote versions. This allows for customization during installation.

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

### Initialize Project with shadcn CLI

Source: https://ui.shadcn.com/docs/changelog

Initializes a new project with shadcn/ui components. The command automatically detects the project's framework and can even set up a new Next.js application. It is a convenient way to start integrating shadcn/ui.

```bash
npx shadcn init
pnpm dlx shadcn init sidebar-01 login-01
```

---

### shadcn CLI Commands

Source: https://ui.shadcn.com/docs/changelog

Demonstrates the usage of the shadcn CLI for managing UI components. The commands shown are 'init' for project setup and 'add' for incorporating new components, along with an experimental 'diff' command. These are essential for integrating and managing shadcn/ui within a project.

```bash
npx shadcn-ui@latest init
npx shadcn-ui@latest add
npx shadcn-ui@latest diff (experimental)
```

---

### Install shadcn/ui Tabs Component

Source: https://ui.shadcn.com/docs/components/tabs

Instructions for installing the Tabs component using pnpm and the shadcn-ui CLI. This ensures the component is correctly added to your project's dependencies and configuration.

```bash
pnpm dlx shadcn@latest add tabs
```

---

### Install shadcn/ui Badge Component (CLI)

Source: https://ui.shadcn.com/docs/components/badge

Command-line instructions for adding the shadcn/ui Badge component to your project using pnpm. This command fetches and installs the necessary component files.

```bash
pnpm dlx shadcn@latest add badge
```

---

### Define a Login Block

Source: https://ui.shadcn.com/docs/registry/examples

This example shows how to define a 'login-01' block, specifying its type, description, dependencies, and the files it comprises. The 'page.tsx' and 'login-form.tsx' are included as part of the block.

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

### Menubar Component Installation (pnpm)

Source: https://ui.shadcn.com/docs/components/menubar

Instructions for installing the Menubar component from shadcn/ui using the pnpm package manager. This command-line interface (CLI) command fetches and adds the necessary component files to your project.

```bash
pnpm dlx shadcn@latest add menubar
```

---

### shadcn/ui Card Component - Installation

Source: https://ui.shadcn.com/docs/components/card

Instructions for installing the Card component in your project using the shadcn/ui CLI. This command automates the process of adding the necessary files to your components directory.

```bash
pnpm dlx shadcn@latest add card
```

---

### Custom Theme Example

Source: https://ui.shadcn.com/docs/registry/examples

Defines a custom theme with specific color variables for both light and dark modes. This allows for a unique color palette across the application.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "custom-theme",
  "type": "registry:theme",
  "cssVars": {
    "light": {
      "background": "oklch(1 0 0)",
      "foreground": "oklch(0.141 0.005 285.823)",
      "primary": "oklch(0.546 0.245 262.881)",
      "primary-foreground": "oklch(0.97 0.014 254.604)",
      "ring": "oklch(0.746 0.16 232.661)",
      "sidebar-primary": "oklch(0.546 0.245 262.881)",
      "sidebar-primary-foreground": "oklch(0.97 0.014 254.604)",
      "sidebar-ring": "oklch(0.746 0.16 232.661)"
    },
    "dark": {
      "background": "oklch(1 0 0)",
      "foreground": "oklch(0.141 0.005 285.823)",
      "primary": "oklch(0.707 0.165 254.624)",
      "primary-foreground": "oklch(0.97 0.014 254.604)",
      "ring": "oklch(0.707 0.165 254.624)",
      "sidebar-primary": "oklch(0.707 0.165 254.624)",
      "sidebar-primary-foreground": "oklch(0.97 0.014 254.604)",
      "sidebar-ring": "oklch(0.707 0.165 254.624)"
    }
  }
}
```

---

### Install Form Component - CLI

Source: https://ui.shadcn.com/docs/components/form

Command to add the Form component to your project using the shadcn-ui CLI. This command installs the necessary dependencies and component files.

```bash
pnpm dlx shadcn@latest add form
```

---

### Navigation Menu Installation (CLI)

Source: https://ui.shadcn.com/docs/components/navigation-menu

Provides instructions for adding the Navigation Menu component to your project using the shadcn/ui CLI with pnpm, npm, or yarn package managers.

```bash
pnpm dlx shadcn@latest add navigation-menu
```

---

### shadcn/ui Popover Installation Commands

Source: https://ui.shadcn.com/docs/components/popover

Provides commands for installing the shadcn/ui Popover component. It includes instructions for pnpm, npm, and yarn package managers, as well as a command to add the Popover component using the shadcn CLI.

```bash
pnpm
npm
yarn
bun
```

```bash
pnpm dlx shadcn@latest add popover
```

---

### Install Calendar Blocks using shadcn-ui CLI

Source: https://ui.shadcn.com/docs/components/calendar

Instructions to install the latest version of the calendar blocks using the shadcn-ui CLI. This command fetches and adds the necessary calendar components to your project.

```bash
pnpm dlx shadcn@latest add calendar-02
```

---

### Initialize Project from Local File using shadcn CLI

Source: https://ui.shadcn.com/docs/changelog

This command initializes a project using a local JSON file as a template. It's useful for zero-setup workflows and testing registry items locally.

```bash
npx shadcn init ./template.json
```

---

### Avatar Component Installation

Source: https://ui.shadcn.com/docs/components/avatar

Provides instructions for installing the Avatar component using the shadcn/ui CLI. This typically involves running a command like 'pnpm dlx shadcn@latest add avatar' after setting up the project.

```bash
pnpm dlx shadcn@latest add avatar
```

---

### shadcn/ui Slider Component Installation

Source: https://ui.shadcn.com/docs/components/slider

Instructions for installing the shadcn/ui Slider component. This includes methods using pnpm, npm, yarn, and bun, as well as a command-line instruction to add the slider dependency using the shadcn CLI.

```bash
pnpm dlx shadcn@latest add slider
```

---

### Install shadcn/ui Button Component

Source: https://ui.shadcn.com/docs/components/button

Instructions for installing the shadcn/ui Button component using package managers like pnpm, npm, or yarn. It also includes the command to add the button component to your project.

```bash
pnpm dlx shadcn@latest add button
```

---

### Install Sidebar Component using PNPM

Source: https://ui.shadcn.com/docs/components/sidebar

Installs the 'sidebar.tsx' component using the pnpm package manager. This is the recommended method for adding the component to your project.

```bash
pnpm dlx shadcn@latest add sidebar
```

---

### Custom Style Extending shadcn/ui

Source: https://ui.shadcn.com/docs/registry/examples

Defines a custom style that extends shadcn/ui, installing specific dependencies and components. It also sets custom CSS variables for fonts and a brand color in both light and dark modes.

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

### Install shadcn/ui Table Component (CLI)

Source: https://ui.shadcn.com/docs/components/table

This command installs the Table component and its dependencies into your shadcn/ui project using the pnpm package manager. Ensure you have shadcn/ui CLI set up.

```bash
pnpm dlx shadcn@latest add table
```

---

### Install Textarea Component with shadcn/ui

Source: https://ui.shadcn.com/docs/components/textarea

Instructions for installing the Textarea component using different package managers (pnpm, npm, yarn, bun). This is a prerequisite for using the Textarea component in your project.

```bash
pnpm dlx shadcn@latest add textarea
```

---

### Create Remix Project

Source: https://ui.shadcn.com/docs/installation/remix

Command to create a new Remix project using pnpm, npm, or yarn. Ensures a fresh project structure for integration.

```bash
pnpm dlx create-remix@latest my-app
```

---

### Sheet Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/sheet

Provides instructions for installing the Sheet component using the shadcn-ui CLI. This command-line approach simplifies the process of adding the component to your project, ensuring all necessary dependencies are handled.

```bash
pnpm dlx shadcn@latest add sheet
```

---

### SidebarHeader Component Example

Source: https://ui.shadcn.com/docs/components/sidebar

Shows how to incorporate the SidebarHeader component to add a sticky header. The example includes a dropdown menu for workspace selection.

```typescript
import { SidebarHeader } from "@/components/ui/sidebar"

export function AppSidebar() {
 return (





 Acme Inc.
 Acme Corp.


 )
}
```

---

### shadcn/ui Drawer Component Installation Command

Source: https://ui.shadcn.com/docs/components/drawer

Shows the command-line interface (CLI) command to install the Drawer component into a shadcn/ui project. This command uses `pnpm dlx` to execute the `shadcn@latest add drawer` command, ensuring the latest version is added.

```bash
pnpm dlx shadcn@latest add drawer

```

---

### Add CSS Import with Media Queries

Source: https://ui.shadcn.com/docs/registry/examples

This configuration shows how to use CSS imports with media queries, allowing for conditional loading of styles based on print or screen size.

```json
{
 "$schema": "https://ui.shadcn.com/schema/registry-item.json",
 "name": "responsive-import",
 "type": "registry:item",
 "css": {
 "@import "print-styles.css" print": {},
 "@import url("mobile.css") screen and (max-width: 768px)": {}
 }
}
```

---

### SidebarFooter Component Example

Source: https://ui.shadcn.com/docs/components/sidebar

Demonstrates the use of the SidebarFooter component for adding a sticky footer. This example includes user account and billing links, and a sign-out option.

```typescript
import { SidebarFooter } from "@/components/ui/sidebar"

export function AppSidebar() {
 return (


CN

shadcnm@example.com

More optionsAccountBillingSettingsSign out
 )
}
```

---

### shadcn/ui Dialog Component Installation (CLI)

Source: https://ui.shadcn.com/docs/components/dialog

Provides the command to install the Dialog component using the shadcn/ui CLI with pnpm. This command adds the necessary files and dependencies for the Dialog component to your project. It's a prerequisite for using the component in your application.

```bash
pnpm dlx shadcn@latest add dialog
```

---

### shadcn CLI init Command Usage

Source: https://ui.shadcn.com/docs/cli

Illustrates the usage of the `shadcn init` command, including its arguments for specifying component names, URLs, or local paths, and available options like `--template`. This command is crucial for setting up the project's components and configuration.

```bash
Usage: shadcn init [options] [components...]

initialize your project and install dependencies

Arguments:
 components name, url or local path to component

Options:
 -t, --template
```

---

### Install shadcn/ui Accordion Component

Source: https://ui.shadcn.com/docs/components/accordion

Provides commands for installing the Accordion component using different package managers (pnpm, yarn, bun). It also shows the command to add the component using the shadcn-ui CLI.

```bash
pnpm dlx shadcn@latest add accordion
```

---

### SidebarMenuAction - DropdownMenu Example

Source: https://ui.shadcn.com/docs/components/sidebar

An example of integrating a DropdownMenu with the SidebarMenuAction component. This allows for contextual actions or sub-options.

```javascript
[Home](#)
Edit Project
Delete Project
```

---

### shadcn/ui Checkbox Installation

Source: https://ui.shadcn.com/docs/components/checkbox

Provides instructions for installing the Checkbox component using different package managers (pnpm, npm, yarn, bun). It also includes the command to add the component using the shadcn-ui CLI.

```bash
pnpm dlx shadcn@latest add checkbox
```

---

### Install Hover Card Component using pnpm

Source: https://ui.shadcn.com/docs/components/hover-card

Command to install the Hover Card component and its dependencies into your project using pnpm and the shadcn-ui CLI.

```bash
pnpm dlx shadcn@latest add hover-card
```

---

### Create React Router Project

Source: https://ui.shadcn.com/docs/installation/react-router

This command initializes a new React Router project using pnpm. It's the first step in setting up your application before integrating shadcn/ui.

```bash
pnpm dlx create-react-router@latest my-app
```

---

### Create Astro Project with Tailwind CSS

Source: https://ui.shadcn.com/docs/installation/astro

Command to create a new Astro project with Tailwind CSS, React integration, package installation, and Git initialization.

```bash
pnpm dlx create-astro@latest astro-app --template with-tailwindcss --install --add react --git
```

---

### Express.js Example for Registry Authentication

Source: https://ui.shadcn.com/docs/registry/authentication

Example of an Express.js route handler for registry authentication, checking the Authorization header for a Bearer token.

````APIDOC
## Express.js Example

### Description
This Express.js example demonstrates a basic route handler for authenticating requests to your component registry. It checks the `Authorization` header for a Bearer token.

### Method
GET

### Endpoint
`/registry/:name.json`

### Parameters
#### Path Parameters
- **name** (string) - Required - The name of the component to retrieve.

#### Query Parameters
N/A

#### Request Body
N/A

### Request Example
N/A (GET request)

### Response
#### Success Response (200)
- **component** (object) - The requested component data.

#### Error Responses
- **401 Unauthorized**: If the provided token is invalid.
- **404 Not Found**: If the component is not found.

### Code Example
```javascript
app.get("/registry/:name.json", (req, res) => {
 const token = req.headers.authorization?.replace("Bearer ", "")

 if (!isValidToken(token)) {
 return res.status(401).json({ error: "Unauthorized" })
 }

 const component = getComponent(req.params.name)
 if (!component) {
 return res.status(404).json({ error: "Component not found" })
 }

 res.json(component)
})

function isValidToken(token) {
 // Add your token validation logic here.
 // Example: return token === process.env.VALID_TOKEN;
 return true; // Placeholder
}

function getComponent(componentName) {
 // Replace with your actual logic to fetch the component.
 // Example: return { name: componentName, data: "component data" };
 return null; // Placeholder
}
````

````

--------------------------------

### shadcn/ui Context Menu Installation (CLI)

Source: https://ui.shadcn.com/docs/components/context-menu

Instructions for adding the shadcn/ui Context Menu component to your project using the command-line interface. This command will automatically download and update the necessary dependencies for the component.

```bash
pnpm dlx shadcn@latest add context-menu
````

---

### Add Complex CSS Utility

Source: https://ui.shadcn.com/docs/registry/examples

This example defines a more complex CSS utility called 'scrollbar-hidden'. It targets the scrollbar pseudo-elements to hide them.

```json
{
  "$schema": "https://ui.shadcn.com/schema/registry-item.json",
  "name": "custom-component",
  "type": "registry:component",
  "css": {
    "@utility scrollbar-hidden": {
      "scrollbar-hidden": {
        "&::-webkit-scrollbar": {
          "display": "none"
        }
      }
    }
  }
}
```

---

### shadcn/ui Label Component Installation

Source: https://ui.shadcn.com/docs/components/label

Provides instructions for installing the Label component into a shadcn/ui project using different package managers like pnpm, npm, yarn, and bun. It also shows the command to add the component using the shadcn CLI.

```bash
pnpm dlx shadcn@latest add label
```

---

### Install Aspect Ratio Component using pnpm

Source: https://ui.shadcn.com/docs/components/aspect-ratio

Command to install the Aspect Ratio component using the pnpm package manager. This command adds the component to your project, making it available for use.

```bash
pnpm dlx shadcn@latest add aspect-ratio
```

---

### React ButtonGroup Installation using pnpm

Source: https://ui.shadcn.com/docs/components/button-group

Provides the command to install the 'button-group' component from shadcn/ui using the pnpm package manager. This command downloads and integrates the necessary files into your project.

```bash
pnpm dlx shadcn@latest add button-group
```

---

### Switch Component Usage Example (React)

Source: https://ui.shadcn.com/docs/components/switch

This code snippet demonstrates how to use the Switch component in a React application. It requires the Label and Switch components from '@/components/ui/label' and '@/components/ui/switch' respectively. The example shows a basic implementation with a label and the switch itself.

```jsx
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"

export function SwitchDemo() {
 return (


Airplane Mode

 )
}
```
