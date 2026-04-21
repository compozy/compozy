# ssr-prerender: Configure Static Prerendering and ISR

## Priority: MEDIUM

## Explanation

Static prerendering generates HTML at build time for pages that don't require request-time data. Incremental Static Regeneration (ISR) extends this by revalidating cached pages on a schedule. Use these for better performance and lower server costs.

## Bad Example

```tsx
// SSR for completely static content - wasteful
export const Route = createFileRoute("/about")({
  loader: async () => {
    // Fetching static content on every request
    const content = await fetchAboutPageContent();
    return { content };
  },
});

// Or no caching headers for semi-static content
export const Route = createFileRoute("/blog/$slug")({
  loader: async ({ params }) => {
    const post = await fetchPost(params.slug);
    return { post };
    // Every request hits the database
  },
});
```

## Good Example: Static Prerendering

```tsx
// app.config.ts
import { defineConfig } from "@tanstack/react-start/config";

export default defineConfig({
  server: {
    prerender: {
      // Routes to prerender at build time
      routes: ["/", "/about", "/contact", "/pricing"],
      // Or crawl from root
      crawlLinks: true,
    },
  },
});

// routes/about.tsx - Will be prerendered
export const Route = createFileRoute("/about")({
  loader: async () => {
    // Runs at BUILD time, not request time
    const content = await fetchAboutPageContent();
    return { content };
  },
  component: AboutPage,
});
```

## Good Example: Dynamic Prerendering

```tsx
// app.config.ts
export default defineConfig({
  server: {
    prerender: {
      // Generate routes dynamically
      routes: async () => {
        const posts = await db.posts.findMany({
          where: { published: true },
          select: { slug: true },
        });

        return ["/", "/blog", ...posts.map(p => `/blog/${p.slug}`)];
      },
    },
  },
});
```

## Good Example: ISR with Revalidation

```tsx
// routes/blog/$slug.tsx
import { createFileRoute } from "@tanstack/react-router";
import { setHeaders } from "@tanstack/react-start/server";

export const Route = createFileRoute("/blog/$slug")({
  loader: async ({ params }) => {
    const post = await fetchPost(params.slug);

    // ISR: Cache for 60 seconds, then revalidate
    setHeaders({
      "Cache-Control": "public, s-maxage=60, stale-while-revalidate=300",
    });

    return { post };
  },
  component: BlogPost,
});

// First request: SSR and cache
// Next 60 seconds: Serve cached version
// After 60 seconds: Serve stale, revalidate in background
// After 300 seconds: Full SSR again
```

## Good Example: Hybrid Static/Dynamic

```tsx
// routes/products.tsx - Prerendered
export const Route = createFileRoute("/products")({
  loader: async () => {
    // Featured products - prerendered at build
    const featured = await fetchFeaturedProducts();
    return { featured };
  },
});

// routes/products/$productId.tsx - ISR
export const Route = createFileRoute("/products/$productId")({
  loader: async ({ params }) => {
    const product = await fetchProduct(params.productId);

    if (!product) throw notFound();

    // Cache product pages for 5 minutes
    setHeaders({
      "Cache-Control": "public, s-maxage=300, stale-while-revalidate=600",
    });

    return { product };
  },
});

// routes/cart.tsx - Always SSR (user-specific)
export const Route = createFileRoute("/cart")({
  loader: async ({ context }) => {
    // No caching - user-specific data
    setHeaders({
      "Cache-Control": "private, no-store",
    });

    const cart = await fetchUserCart(context.user.id);
    return { cart };
  },
});
```

## Good Example: On-Demand Revalidation

```tsx
// API route to trigger revalidation
// app/routes/api/revalidate.ts
export const APIRoute = createAPIFileRoute("/api/revalidate")({
  POST: async ({ request }) => {
    const { secret, path } = await request.json();

    // Verify secret
    if (secret !== process.env.REVALIDATE_SECRET) {
      return json({ error: "Invalid secret" }, { status: 401 });
    }

    // Trigger revalidation (implementation depends on hosting)
    await revalidatePath(path);

    return json({ revalidated: true, path });
  },
});

// Usage: POST /api/revalidate { "secret": "...", "path": "/blog/my-post" }
```

## Cache-Control Directives

| Directive                  | Meaning                            |
| -------------------------- | ---------------------------------- |
| `s-maxage=N`               | CDN cache duration (seconds)       |
| `max-age=N`                | Browser cache duration             |
| `stale-while-revalidate=N` | Serve stale while fetching fresh   |
| `private`                  | Don't cache on CDN (user-specific) |
| `no-store`                 | Never cache                        |

## Context

- Prerendering happens at build time - no request context
- ISR requires CDN/edge support (Vercel, Cloudflare, etc.)
- Use prerendering for truly static pages (about, pricing)
- Use ISR for content that changes but not per-request
- Always SSR for user-specific or real-time data
- Test with production builds - dev server is always SSR
