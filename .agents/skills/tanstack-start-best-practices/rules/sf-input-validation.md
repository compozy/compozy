# sf-input-validation: Always Validate Server Function Inputs

## Priority: CRITICAL

## Explanation

Server functions receive data across the network boundary. Always validate inputs before processing - never trust client data. Use schema validation libraries like Zod for type-safe validation.

## Bad Example

```tsx
// No validation - trusting client input directly
export const updateUser = createServerFn({ method: "POST" }).handler(async ({ data }) => {
  // data is unknown/any - no type safety
  // SQL injection, invalid data, type errors all possible
  await db.users.update({
    where: { id: data.id },
    data: {
      name: data.name,
      email: data.email,
      role: data.role, // Could be set to 'admin' by malicious client!
    },
  });
});

// Weak validation - type assertion without runtime check
export const deletePost = createServerFn({ method: "POST" }).handler(
  async ({ data }: { data: { id: string } }) => {
    // Type assertion doesn't validate at runtime
    await db.posts.delete({ where: { id: data.id } });
  }
);
```

## Good Example: With Zod Validation

```tsx
import { createServerFn } from "@tanstack/react-start";
import { z } from "zod";

const updateUserSchema = z.object({
  id: z.string().uuid(),
  name: z.string().min(1).max(100),
  email: z.string().email(),
  // Don't allow role updates from client input!
});

export const updateUser = createServerFn({ method: "POST" })
  .validator(updateUserSchema)
  .handler(async ({ data }) => {
    // data is fully typed: { id: string; name: string; email: string }
    const user = await db.users.update({
      where: { id: data.id },
      data: {
        name: data.name,
        email: data.email,
      },
    });
    return user;
  });

// Validation errors are automatically returned to client
// with proper status codes and messages
```

## Good Example: Complex Validation

```tsx
const createOrderSchema = z.object({
  items: z
    .array(
      z.object({
        productId: z.string().uuid(),
        quantity: z.number().int().min(1).max(100),
      })
    )
    .min(1)
    .max(50),
  shippingAddress: z.object({
    street: z.string().min(1),
    city: z.string().min(1),
    state: z.string().length(2),
    zip: z.string().regex(/^\d{5}(-\d{4})?$/),
  }),
  couponCode: z.string().optional(),
});

export const createOrder = createServerFn({ method: "POST" })
  .validator(createOrderSchema)
  .handler(async ({ data }) => {
    // All data is validated and typed
    // Process order safely
  });
```

## Good Example: Transform and Refine

```tsx
const registrationSchema = z
  .object({
    email: z.string().email().toLowerCase(), // Transform to lowercase
    password: z
      .string()
      .min(8, "Password must be at least 8 characters")
      .regex(/[A-Z]/, "Password must contain uppercase letter")
      .regex(/[0-9]/, "Password must contain number"),
    confirmPassword: z.string(),
  })
  .refine(data => data.password === data.confirmPassword, {
    message: "Passwords must match",
    path: ["confirmPassword"],
  });

export const register = createServerFn({ method: "POST" })
  .validator(registrationSchema)
  .handler(async ({ data }) => {
    // Passwords match, email is lowercase
    // Only password needed (confirmPassword was for validation)
    const hashedPassword = await hashPassword(data.password);
    return await createUser({
      email: data.email,
      password: hashedPassword,
    });
  });
```

## Sharing Schemas Between Client and Server

```tsx
// lib/schemas/post.ts - Shared validation schema
import { z } from "zod";

export const createPostSchema = z.object({
  title: z.string().min(1).max(200),
  content: z.string().min(1),
  tags: z.array(z.string()).max(10).optional(),
});

export type CreatePostInput = z.infer<typeof createPostSchema>;

// lib/posts.functions.ts - Server function
import { createPostSchema } from "./schemas/post";

export const createPost = createServerFn({ method: "POST" })
  .validator(createPostSchema)
  .handler(async ({ data }) => {
    /* ... */
  });

// components/CreatePostForm.tsx - Client form validation
import { createPostSchema, type CreatePostInput } from "@/lib/schemas/post";

function CreatePostForm() {
  const form = useForm<CreatePostInput>({
    resolver: zodResolver(createPostSchema),
  });
  // Same validation client and server side
}
```

## Context

- Network boundary = trust boundary - always validate
- Use `.validator()` before `.handler()` in the chain
- Validation errors return proper HTTP status codes
- Share schemas between client forms and server functions
- Strip or ignore fields clients shouldn't control (like `role`, `isAdmin`)
- Consider rate limiting for mutation endpoints
