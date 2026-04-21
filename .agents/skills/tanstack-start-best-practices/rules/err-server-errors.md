# err-server-errors: Handle Server Function Errors

## Priority: MEDIUM

## Explanation

Server function errors cross the network boundary. Handle them gracefully with appropriate error types, status codes, and user-friendly messages. Avoid exposing internal details in production.

## Bad Example

```tsx
// Throwing raw errors - exposes internals
export const createUser = createServerFn({ method: "POST" })
  .validator(createUserSchema)
  .handler(async ({ data }) => {
    const user = await db.users.create({ data }); // May throw DB error
    return user;
    // Prisma error with stack trace sent to client
  });

// Generic error handling - no useful info for client
export const getPost = createServerFn().handler(async ({ data }) => {
  try {
    return await fetchPost(data.id);
  } catch (e) {
    throw new Error("Something went wrong"); // Too vague
  }
});
```

## Good Example: Structured Error Handling

```tsx
// lib/errors.ts
export class AppError extends Error {
  constructor(
    message: string,
    public code: string,
    public status: number = 400
  ) {
    super(message);
    this.name = "AppError";
  }
}

export class NotFoundError extends AppError {
  constructor(resource: string) {
    super(`${resource} not found`, "NOT_FOUND", 404);
  }
}

export class UnauthorizedError extends AppError {
  constructor(message = "Unauthorized") {
    super(message, "UNAUTHORIZED", 401);
  }
}

export class ValidationError extends AppError {
  constructor(
    message: string,
    public fields?: Record<string, string>
  ) {
    super(message, "VALIDATION_ERROR", 400);
  }
}
```

## Good Example: Server Function with Error Handling

```tsx
import { createServerFn, notFound } from "@tanstack/react-start";
import { setResponseStatus } from "@tanstack/react-start/server";

export const getPost = createServerFn()
  .validator(z.object({ id: z.string() }))
  .handler(async ({ data }) => {
    const post = await db.posts.findUnique({
      where: { id: data.id },
    });

    if (!post) {
      // Use built-in notFound for 404s
      throw notFound();
    }

    return post;
  });

export const createPost = createServerFn({ method: "POST" })
  .validator(createPostSchema)
  .handler(async ({ data }) => {
    try {
      const post = await db.posts.create({ data });
      return post;
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError) {
        if (error.code === "P2002") {
          // Unique constraint violation
          setResponseStatus(409);
          throw new AppError("A post with this title already exists", "DUPLICATE", 409);
        }
      }

      // Log full error server-side
      console.error("Failed to create post:", error);

      // Return sanitized error to client
      setResponseStatus(500);
      throw new AppError("Failed to create post", "INTERNAL_ERROR", 500);
    }
  });
```

## Good Example: Client-Side Error Handling

```tsx
function CreatePostForm() {
  const [error, setError] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: createPost,
    onError: error => {
      if (error instanceof AppError) {
        setError(error.message);
      } else if (error instanceof ValidationError) {
        // Handle field-specific errors
        Object.entries(error.fields ?? {}).forEach(([field, message]) => {
          form.setError(field, { message });
        });
      } else {
        setError("An unexpected error occurred");
      }
    },
    onSuccess: post => {
      navigate({ to: "/posts/$postId", params: { postId: post.id } });
    },
  });

  return (
    <form onSubmit={handleSubmit}>
      {error && <Alert variant="error">{error}</Alert>}
      {/* form fields */}
    </form>
  );
}
```

## Good Example: Using Redirects for Auth Errors

```tsx
export const updateProfile = createServerFn({ method: "POST" })
  .validator(updateProfileSchema)
  .handler(async ({ data }) => {
    const session = await getSessionData();

    if (!session) {
      // Redirect to login for auth errors
      throw redirect({
        to: "/login",
        search: { redirect: "/settings" },
      });
    }

    return await db.users.update({
      where: { id: session.userId },
      data,
    });
  });
```

## Error Response Best Practices

| Scenario             | HTTP Status | Response                     |
| -------------------- | ----------- | ---------------------------- |
| Validation failed    | 400         | Field-specific errors        |
| Not authenticated    | 401         | Redirect to login            |
| Not authorized       | 403         | Generic forbidden message    |
| Resource not found   | 404         | Use `notFound()`             |
| Conflict (duplicate) | 409         | Specific conflict message    |
| Server error         | 500         | Generic message, log details |

## Context

- Use `notFound()` for 404 errors - integrates with router
- Use `redirect()` for auth-related errors
- Set status codes with `setResponseStatus()`
- Log full errors server-side, sanitize for client
- Create custom error classes for consistent handling
- Validation errors from `.validator()` are automatic
