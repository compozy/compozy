# env-functions: Use Environment Functions for Configuration

## Priority: MEDIUM

## Explanation

Environment functions provide type-safe access to environment variables on the server. They ensure secrets stay server-side, provide validation, and enable different configurations per environment (development, staging, production).

## Bad Example

```tsx
// Accessing env vars directly - no validation, potential leaks
export const getApiData = createServerFn().handler(async () => {
  // No validation - may be undefined
  const apiKey = process.env.API_KEY;

  // Accidentally exposed in error messages
  if (!apiKey) {
    throw new Error(`Missing API_KEY: ${process.env}`);
  }

  return fetch(url, { headers: { Authorization: apiKey } });
});

// Or importing env in shared files
// lib/config.ts
export const config = {
  apiKey: process.env.API_KEY, // Bundled into client!
  dbUrl: process.env.DATABASE_URL,
};
```

## Good Example: Validated Environment Configuration

```tsx
// lib/env.server.ts
import { z } from "zod";

const envSchema = z.object({
  // Required
  DATABASE_URL: z.string().url(),
  SESSION_SECRET: z.string().min(32),

  // API Keys
  STRIPE_SECRET_KEY: z.string().startsWith("sk_"),
  STRIPE_WEBHOOK_SECRET: z.string().startsWith("whsec_"),

  // Optional with defaults
  NODE_ENV: z.enum(["development", "staging", "production"]).default("development"),
  LOG_LEVEL: z.enum(["debug", "info", "warn", "error"]).default("info"),

  // Optional
  SENTRY_DSN: z.string().url().optional(),
});

export type Env = z.infer<typeof envSchema>;

function validateEnv(): Env {
  const parsed = envSchema.safeParse(process.env);

  if (!parsed.success) {
    console.error("Invalid environment variables:");
    console.error(parsed.error.flatten().fieldErrors);
    throw new Error("Invalid environment configuration");
  }

  return parsed.data;
}

// Validate once at startup
export const env = validateEnv();

// Usage in server functions
export const getPaymentIntent = createServerFn({ method: "POST" }).handler(async () => {
  const stripe = new Stripe(env.STRIPE_SECRET_KEY);
  // Type-safe, validated access
});
```

## Good Example: Public vs Private Config

```tsx
// lib/env.server.ts - Server only (secrets)
export const serverEnv = {
  databaseUrl: process.env.DATABASE_URL!,
  sessionSecret: process.env.SESSION_SECRET!,
  stripeSecretKey: process.env.STRIPE_SECRET_KEY!,
};

// lib/env.ts - Public config (safe for client)
export const publicEnv = {
  appUrl: process.env.VITE_APP_URL ?? "http://localhost:3000",
  stripePublicKey: process.env.VITE_STRIPE_PUBLIC_KEY!,
  sentryDsn: process.env.VITE_SENTRY_DSN,
};

// Vite exposes VITE_ prefixed vars to client
// Non-prefixed vars are server-only
```

## Good Example: Environment-Specific Behavior

```tsx
// lib/env.server.ts
export const env = validateEnv();

export const isDevelopment = env.NODE_ENV === "development";
export const isProduction = env.NODE_ENV === "production";
export const isStaging = env.NODE_ENV === "staging";

// lib/logger.server.ts
import { env, isDevelopment } from "./env.server";

export function log(level: string, message: string, data?: unknown) {
  if (isDevelopment) {
    console.log(`[${level}]`, message, data);
    return;
  }

  // Production: send to logging service
  if (env.SENTRY_DSN) {
    // Send to Sentry
  }
}

// Server function with environment checks
export const debugInfo = createServerFn().handler(async () => {
  if (isProduction) {
    throw new Error("Debug endpoint not available in production");
  }

  return {
    nodeVersion: process.version,
    env: env.NODE_ENV,
  };
});
```

## Good Example: Feature Flags via Environment

```tsx
// lib/features.server.ts
import { env } from "./env.server";

export const features = {
  newCheckout: env.FEATURE_NEW_CHECKOUT === "true",
  betaDashboard: env.FEATURE_BETA_DASHBOARD === "true",
  aiAssistant: env.FEATURE_AI_ASSISTANT === "true",
};

// Usage in server functions
export const getCheckoutUrl = createServerFn().handler(async () => {
  if (features.newCheckout) {
    return "/checkout/v2";
  }
  return "/checkout";
});

// Usage in loaders
export const Route = createFileRoute("/dashboard")({
  loader: async () => {
    return {
      showBetaFeatures: features.betaDashboard,
    };
  },
});
```

## Good Example: Type-Safe env.d.ts

```tsx
// env.d.ts - TypeScript declarations for env vars
declare namespace NodeJS {
  interface ProcessEnv {
    // Required
    DATABASE_URL: string;
    SESSION_SECRET: string;

    // Optional
    NODE_ENV?: "development" | "staging" | "production";
    SENTRY_DSN?: string;

    // Vite public vars
    VITE_APP_URL?: string;
    VITE_STRIPE_PUBLIC_KEY: string;
  }
}
```

## Environment Variable Checklist

| Variable                 | Prefix  | Accessible On   |
| ------------------------ | ------- | --------------- |
| `DATABASE_URL`           | None    | Server only     |
| `SESSION_SECRET`         | None    | Server only     |
| `STRIPE_SECRET_KEY`      | None    | Server only     |
| `VITE_APP_URL`           | `VITE_` | Server + Client |
| `VITE_STRIPE_PUBLIC_KEY` | `VITE_` | Server + Client |

## Context

- Never import `.server.ts` files in client code
- Use `VITE_` prefix for client-accessible variables
- Validate at startup to fail fast on misconfiguration
- Use Zod or similar for runtime validation
- Keep secrets out of error messages and logs
- Consider using `.env.local` for local overrides (gitignored)
