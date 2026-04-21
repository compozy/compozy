---
name: tech-logos
description: Install official tech brand logos from the Elements registry. Use when user needs logos for tech companies (Clerk, Vercel, GitHub, etc.), AI providers (OpenAI, Anthropic, Claude), social platforms, or any brand assets. Triggers on "logo", "brand", "icon for [company]", "add [company] logo", placeholder logo detection, or when building landing pages, auth UIs, or integrations showcases.
---

# Tech Logos

Install official, theme-aware brand logos from the Elements registry.

## Install Pattern

```bash
npx shadcn@latest add @elements/{name}-logo
```

Examples: `clerk-logo`, `github-logo`, `openai-logo`, `vercel-logo`

## Discover Available Logos

**Option A**: Scan registry (if in elements repo)

```bash
ls registry/default/blocks/logos/ | sed 's/-logo$//'
```

**Option B**: Browse https://tryelements.dev/docs/logos

## After Install

Logos install to `components/logos/{name}.tsx`:

```tsx
import { ClerkLogo } from "@/components/logos/clerk"

<ClerkLogo className="h-8 w-auto" />
<ClerkLogo variant="wordmark" mode="dark" />
```

## Common Props

- `variant`: `"icon"` | `"wordmark"`
- `mode`: `"light"` | `"dark"` (auto-detects theme)
- `className`: Standard className prop

## Bundles

| Need             | Command                      |
| ---------------- | ---------------------------- |
| All logos        | `@elements/logos`            |
| AI providers     | `@elements/ai-services`      |
| Social platforms | `@elements/social-media`     |
| Package managers | `@elements/package-managers` |

## Quick Patterns

```bash
# Auth stack
npx shadcn@latest add @elements/clerk-logo @elements/better-auth-logo

# AI models
npx shadcn@latest add @elements/openai-logo @elements/anthropic-logo @elements/claude-logo

# Social footer
npx shadcn@latest add @elements/twitter-logo @elements/github-logo @elements/discord-logo

# Tech stack
npx shadcn@latest add @elements/vercel-logo @elements/supabase-logo @elements/stripe-logo
```

## Logo Not Found?

If the logo doesn't exist in the registry, help the user request it:

**Generate a pre-filled GitHub issue URL:**

```
https://github.com/crafter-station/elements/issues/new?title=[Logo%20Request]%20Add%20{Name}%20logo&body=...&labels=enhancement,logo-request
```

**Example for "Neon" logo:**

```
https://github.com/crafter-station/elements/issues/new?title=%5BLogo%20Request%5D%20Add%20Neon%20logo&body=%23%23%20Logo%20Request%0A%0A**Company%2FService%3A**%20Neon%0A**Website%3A**%20https%3A%2F%2Fneon.tech%0A%0A%23%23%20Why%20this%20logo%3F%0A%3C!--%20Brief%20description%20--%3E%0A%0A---%0A*Auto-generated%20from%20tech-logos%20skill*&labels=enhancement,logo-request
```

**Tell the user:**

> "The {Name} logo isn't available yet. [Click here to request it]({issue_url}) - the issue is pre-filled!"
