# sf-create-server-fn: Use createServerFn for Server-Side Logic

## Priority: CRITICAL

## Explanation

`createServerFn()` creates type-safe server functions that can be called from anywhere - loaders, components, or other server functions. The code inside the handler runs only on the server, with automatic RPC for client calls.

## Bad Example

```tsx
// Using fetch directly - no type safety, manual serialization
async function createPost(data: CreatePostInput) {
  const response = await fetch("/api/posts", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!response.ok) throw new Error("Failed to create post");
  return response.json();
}

// Or using API routes - more boilerplate
// api/posts.ts
export async function POST(request: Request) {
  const data = await request.json();
  // No type safety from client
  const post = await db.posts.create({ data });
  return new Response(JSON.stringify(post));
}
```

## Good Example

```tsx
// lib/posts.functions.ts
import { createServerFn } from "@tanstack/react-start";
import { z } from "zod";
import { db } from "./db.server";

const createPostSchema = z.object({
  title: z.string().min(1).max(200),
  content: z.string().min(1),
  published: z.boolean().default(false),
});

export const createPost = createServerFn({ method: "POST" })
  .validator(createPostSchema)
  .handler(async ({ data }) => {
    // This code only runs on the server
    const post = await db.posts.create({
      data: {
        title: data.title,
        content: data.content,
        published: data.published,
      },
    });
    return post;
  });

// Usage in component
function CreatePostForm() {
  const createPostMutation = useServerFn(createPost);

  const handleSubmit = async (formData: FormData) => {
    try {
      const post = await createPostMutation({
        data: {
          title: formData.get("title") as string,
          content: formData.get("content") as string,
          published: false,
        },
      });
      // post is fully typed
      console.log("Created post:", post.id);
    } catch (error) {
      console.error("Failed to create post:", error);
    }
  };
}
```

## Good Example: GET Function for Data Fetching

```tsx
// lib/posts.functions.ts
export const getPosts = createServerFn() // GET is default
  .handler(async () => {
    const posts = await db.posts.findMany({
      orderBy: { createdAt: "desc" },
      take: 20,
    });
    return posts;
  });

export const getPost = createServerFn()
  .validator(z.object({ id: z.string() }))
  .handler(async ({ data }) => {
    const post = await db.posts.findUnique({
      where: { id: data.id },
    });
    if (!post) {
      throw notFound();
    }
    return post;
  });

// Usage in route loader
export const Route = createFileRoute("/posts/$postId")({
  loader: async ({ params }) => {
    return await getPost({ data: { id: params.postId } });
  },
});
```

## Good Example: With Context and Dependencies

```tsx
// Compose server functions
export const getPostWithComments = createServerFn()
  .validator(z.object({ postId: z.string() }))
  .handler(async ({ data }) => {
    const [post, comments] = await Promise.all([
      getPost({ data: { id: data.postId } }),
      getComments({ data: { postId: data.postId } }),
    ]);

    return { post, comments };
  });
```

## Key Benefits

- **Type safety**: Input/output types flow through client and server
- **Automatic serialization**: No manual JSON parsing
- **Code splitting**: Server code never reaches client bundle
- **Composable**: Call from loaders, components, or other server functions
- **Validation**: Built-in input validation with schema libraries

## Context

- Default method is GET (idempotent, cacheable)
- Use POST for mutations that change data
- Server functions are RPC calls under the hood
- Validation errors are properly typed and serialized
- Import is safe on client - build process replaces with RPC stub
