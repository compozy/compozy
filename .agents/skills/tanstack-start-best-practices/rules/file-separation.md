# file-separation: Separate Server and Client Code

## Priority: LOW

## Explanation

Organize code by execution context to prevent server code from accidentally bundling into client builds. Use `.server.ts` for server-only code, `.functions.ts` for server function definitions, and standard `.ts` for shared code.

## Bad Example

```tsx
// lib/posts.ts - Mixed server and client code
import { db } from "./db"; // Database - server only
import { formatDate } from "./utils"; // Utility - shared

export async function getPosts() {
  // This uses db, so it's server-only
  // But file might be imported on client
  return db.posts.findMany();
}

export function formatPostDate(date: Date) {
  // This could run anywhere
  return formatDate(date);
}

// routes/posts.tsx
import { getPosts, formatPostDate } from "@/lib/posts";
// Importing getPosts pulls db into client bundle (error or bloat)
```

## Good Example: Clear Separation

```
lib/
├── posts.ts              # Shared types and utilities
├── posts.server.ts       # Server-only database logic
├── posts.functions.ts    # Server function definitions
└── schemas/
    └── post.ts           # Shared validation schemas
```

```tsx
// lib/posts.ts - Shared (safe to import anywhere)
export interface Post {
  id: string;
  title: string;
  content: string;
  createdAt: Date;
}

export function formatPostDate(date: Date): string {
  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "medium",
  }).format(date);
}

// lib/posts.server.ts - Server only (never import on client)
import { db } from "./db";
import type { Post } from "./posts";

export async function getPostsFromDb(): Promise<Post[]> {
  return db.posts.findMany({
    orderBy: { createdAt: "desc" },
  });
}

export async function createPostInDb(data: CreatePostInput): Promise<Post> {
  return db.posts.create({ data });
}

// lib/posts.functions.ts - Server functions (safe to import anywhere)
import { createServerFn } from "@tanstack/react-start";
import { getPostsFromDb, createPostInDb } from "./posts.server";
import { createPostSchema } from "./schemas/post";

export const getPosts = createServerFn().handler(async () => {
  return await getPostsFromDb();
});

export const createPost = createServerFn({ method: "POST" })
  .validator(createPostSchema)
  .handler(async ({ data }) => {
    return await createPostInDb(data);
  });
```

## Good Example: Using in Components

```tsx
// components/PostList.tsx
import { getPosts } from "@/lib/posts.functions"; // Safe - RPC stub on client
import { formatPostDate } from "@/lib/posts"; // Safe - shared utility
import type { Post } from "@/lib/posts"; // Safe - type only

function PostList() {
  const postsQuery = useQuery({
    queryKey: ["posts"],
    queryFn: () => getPosts(), // Calls server function
  });

  return (
    <ul>
      {postsQuery.data?.map(post => (
        <li key={post.id}>
          {post.title}
          <span>{formatPostDate(post.createdAt)}</span>
        </li>
      ))}
    </ul>
  );
}
```

## File Convention Summary

| Suffix          | Purpose                         | Safe to Import on Client |
| --------------- | ------------------------------- | ------------------------ |
| `.ts`           | Shared utilities, types         | Yes                      |
| `.server.ts`    | Server-only logic (db, secrets) | No                       |
| `.functions.ts` | Server function wrappers        | Yes                      |
| `.client.ts`    | Client-only code                | Yes (client only)        |

## Good Example: Environment Variables

```tsx
// lib/config.server.ts - Server secrets
export const config = {
  databaseUrl: process.env.DATABASE_URL!,
  sessionSecret: process.env.SESSION_SECRET!,
  stripeSecretKey: process.env.STRIPE_SECRET_KEY!,
};

// lib/config.ts - Public config (safe for client)
export const publicConfig = {
  appName: "My App",
  apiUrl: process.env.NEXT_PUBLIC_API_URL,
  stripePublicKey: process.env.NEXT_PUBLIC_STRIPE_KEY,
};

// Never import config.server.ts on client
```

## Context

- `.server.ts` files should never be directly imported in client code
- Server functions in `.functions.ts` are safe - build replaces with RPC
- Types from `.server.ts` are safe if using `import type`
- TanStack Start's build process validates proper separation
- This pattern enables tree-shaking and smaller client bundles
- Use consistent naming convention across your team
