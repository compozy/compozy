# mw-request-middleware: Use Request Middleware for Cross-Cutting Concerns

## Priority: HIGH

## Explanation

Request middleware runs before every server request (routes, SSR, server functions). Use it for authentication, logging, rate limiting, and other cross-cutting concerns that apply globally.

## Bad Example

```tsx
// Duplicating auth logic in every server function
export const getProfile = createServerFn().handler(async () => {
  const session = await getSession();
  if (!session) throw new Error("Unauthorized");
  // ... rest of handler
});

export const updateProfile = createServerFn({ method: "POST" }).handler(async ({ data }) => {
  const session = await getSession();
  if (!session) throw new Error("Unauthorized");
  // ... rest of handler
});

export const deleteAccount = createServerFn({ method: "POST" }).handler(async () => {
  const session = await getSession();
  if (!session) throw new Error("Unauthorized");
  // ... rest of handler
});
```

## Good Example: Authentication Middleware

```tsx
// lib/middleware/auth.ts
import { createMiddleware } from "@tanstack/react-start";
import { getSession } from "./session.server";

export const authMiddleware = createMiddleware().server(async ({ next }) => {
  const session = await getSession();

  // Pass session to downstream handlers via context
  return next({
    context: {
      session,
      user: session?.user ?? null,
    },
  });
});

// lib/middleware/requireAuth.ts
export const requireAuthMiddleware = createMiddleware()
  .middleware([authMiddleware]) // Depends on auth middleware
  .server(async ({ next, context }) => {
    if (!context.user) {
      throw redirect({ to: "/login" });
    }

    return next({
      context: {
        user: context.user, // Now guaranteed to exist
      },
    });
  });
```

## Good Example: Logging Middleware

```tsx
// lib/middleware/logging.ts
export const loggingMiddleware = createMiddleware().server(async ({ next, request }) => {
  const start = Date.now();
  const requestId = crypto.randomUUID();

  console.log(`[${requestId}] ${request.method} ${request.url}`);

  try {
    const result = await next({
      context: { requestId },
    });

    console.log(`[${requestId}] Completed in ${Date.now() - start}ms`);
    return result;
  } catch (error) {
    console.error(`[${requestId}] Error:`, error);
    throw error;
  }
});
```

## Good Example: Global Middleware Configuration

```tsx
// app/start.ts
import { createStart } from "@tanstack/react-start/server";
import { loggingMiddleware } from "./middleware/logging";
import { authMiddleware } from "./middleware/auth";

export default createStart({
  // Request middleware runs for all requests
  requestMiddleware: [loggingMiddleware, authMiddleware],
});
```

## Good Example: Rate Limiting Middleware

```tsx
// lib/middleware/rateLimit.ts
import { createMiddleware } from "@tanstack/react-start";

const rateLimitStore = new Map<string, { count: number; resetAt: number }>();

export const rateLimitMiddleware = createMiddleware().server(async ({ next, request }) => {
  const ip = request.headers.get("x-forwarded-for") ?? "unknown";
  const now = Date.now();
  const windowMs = 60 * 1000; // 1 minute
  const maxRequests = 100;

  let record = rateLimitStore.get(ip);

  if (!record || record.resetAt < now) {
    record = { count: 0, resetAt: now + windowMs };
  }

  record.count++;
  rateLimitStore.set(ip, record);

  if (record.count > maxRequests) {
    throw new Response("Too Many Requests", { status: 429 });
  }

  return next();
});
```

## Middleware Execution Order

```
Request → Middleware 1 → Middleware 2 → Handler → Middleware 2 → Middleware 1 → Response

// Example with timing:
loggingMiddleware.server(async ({ next }) => {
  console.log('Before handler')
  const result = await next()  // Calls next middleware/handler
  console.log('After handler')
  return result
})
```

## Context

- Request middleware applies to all server requests
- Middleware can add to context using `next({ context: {...} })`
- Order matters - first middleware wraps the entire chain
- Global middleware defined in `app/start.ts`
- Route-specific middleware uses `beforeLoad`
- Server function middleware uses separate pattern (see `mw-function-middleware`)
