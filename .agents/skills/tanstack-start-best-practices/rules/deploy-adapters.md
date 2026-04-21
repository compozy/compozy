# deploy-adapters: Choose Appropriate Deployment Adapter

## Priority: LOW

## Explanation

TanStack Start uses deployment adapters to target different hosting platforms. Each adapter optimizes the build output for its platform's runtime, edge functions, and static hosting capabilities.

## Bad Example

```tsx
// Not configuring adapter - using defaults may not match your host
// app.config.ts
export default defineConfig({
  // No adapter specified
  // May not work correctly on your deployment platform
});

// Or using wrong adapter for platform
export default defineConfig({
  server: {
    preset: "node-server", // But deploying to Vercel Edge
  },
});
```

## Good Example: Vercel Deployment

```tsx
// app.config.ts
import { defineConfig } from '@tanstack/react-start/config'

export default defineConfig({
  server: {
    preset: 'vercel',
    // Vercel-specific options
  },
})

// vercel.json (optional, for customization)
{
  "framework": null,
  "buildCommand": "npm run build",
  "outputDirectory": ".output"
}
```

## Good Example: Cloudflare Pages

```tsx
// app.config.ts
import { defineConfig } from "@tanstack/react-start/config";

export default defineConfig({
  server: {
    preset: "cloudflare-pages",
  },
});

// wrangler.toml
name = "my-tanstack-app";
compatibility_date = "2024-01-01";
pages_build_output_dir = ".output/public";

// For Cloudflare Workers (full control)
export default defineConfig({
  server: {
    preset: "cloudflare",
  },
});
```

## Good Example: Netlify

```tsx
// app.config.ts
import { defineConfig } from "@tanstack/react-start/config";

export default // netlify.toml
defineConfig({
  server: {
    preset: "netlify",
  },
})[build];
command = "npm run build";
publish = ".output/public"[functions];
directory = ".output/server";
```

## Good Example: Node.js Server

```tsx
// app.config.ts
import { defineConfig } from '@tanstack/react-start/config'

export default defineConfig({
  server: {
    preset: 'node-server',
    // Optional: customize port
  },
})

// Dockerfile
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY .output .output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]

// Or run directly
// node .output/server/index.mjs
```

## Good Example: Static Export (SPA)

```tsx
// app.config.ts
import { defineConfig } from "@tanstack/react-start/config";

export default defineConfig({
  server: {
    preset: "static",
    prerender: {
      routes: ["/"],
      crawlLinks: true,
    },
  },
});

// Output: .output/public (static files only)
// Host anywhere: GitHub Pages, S3, any static host
```

## Good Example: AWS Lambda

```tsx
// app.config.ts
import { defineConfig } from '@tanstack/react-start/config'

export default defineConfig({
  server: {
    preset: 'aws-lambda',
  },
})

// Deploy with SST, Serverless Framework, or AWS CDK
// serverless.yml example:
service: my-tanstack-app
provider:
  name: aws
  runtime: nodejs20.x
functions:
  app:
    handler: .output/server/index.handler
    events:
      - http: ANY /
      - http: ANY /{proxy+}
```

## Good Example: Bun Runtime

```tsx
// app.config.ts
import { defineConfig } from "@tanstack/react-start/config";

export default defineConfig({
  server: {
    preset: "bun",
  },
});

// Run with: bun .output/server/index.mjs
```

## Adapter Comparison

| Adapter            | Runtime   | Edge | Static | Best For               |
| ------------------ | --------- | ---- | ------ | ---------------------- |
| `vercel`           | Node/Edge | Yes  | Yes    | Vercel hosting         |
| `cloudflare-pages` | Workers   | Yes  | Yes    | Cloudflare Pages       |
| `cloudflare`       | Workers   | Yes  | No     | Cloudflare Workers     |
| `netlify`          | Node      | Yes  | Yes    | Netlify hosting        |
| `node-server`      | Node      | No   | No     | Docker, VPS, self-host |
| `static`           | None      | No   | Yes    | Any static host        |
| `aws-lambda`       | Node      | No   | No     | AWS serverless         |
| `bun`              | Bun       | No   | No     | Bun runtime            |

## Context

- Adapters transform output for target platform
- Edge adapters have API limitations (no file system, etc.)
- Static preset requires all routes to be prerenderable
- Test locally with `npm run build && npm run preview`
- Check platform docs for runtime-specific constraints
- Some platforms auto-detect TanStack Start (no adapter needed)
