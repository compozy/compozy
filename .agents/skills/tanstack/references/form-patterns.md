# TanStack Form Patterns and Best Practices

This document contains comprehensive patterns and best practices for working with TanStack Form in this codebase.

## Table of Contents

1. [Core Concepts](#core-concepts)
2. [Form Hook Setup](#form-hook-setup)
3. [Validation Patterns](#validation-patterns)
4. [Field Components](#field-components)
5. [Form State Management](#form-state-management)
6. [Array Fields](#array-fields)
7. [Error Handling](#error-handling)
8. [Performance Optimization](#performance-optimization)
9. [Anti-Patterns to Avoid](#anti-patterns-to-avoid)

---

## Core Concepts

### Type Safety

TanStack Form automatically infers types from default values. Avoid manual generic declarations:

```typescript
// GOOD: Let TanStack Form infer types from defaultValues
const form = useForm({
  defaultValues: {
    username: "",
    email: "",
    age: 0,
  },
});

// BAD: Don't manually specify generics
const form = useForm<{ username: string; email: string; age: number }>({
  defaultValues: {
    username: "",
    email: "",
    age: 0,
  },
});
```

### Headless Design

TanStack Form provides logic, not markup. Build UI components to match your design system using the provided field state and handlers.

---

## Form Hook Setup

### Using `createFormHook` (Recommended for Production)

For consistency and reduced boilerplate, use `createFormHook` to create a custom form hook with pre-registered components:

```typescript
import { createFormHook, createFormHookContexts } from "@tanstack/react-form";

// Create contexts for field and form access
export const { fieldContext, formContext, useFieldContext, useFormContext } =
  createFormHookContexts();

// Create the custom form hook with registered components
export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: {
    TextField,
    SelectField,
  },
  formComponents: {
    SubmitButton,
  },
});
```

### Using Raw `useForm`

For simpler forms or when custom hook setup is not needed:

```typescript
import { useForm } from "@tanstack/react-form";

const form = useForm({
  defaultValues: {
    name: "",
  },
  onSubmit: async ({ value }) => {
    // Handle submission
  },
});
```

### Form Initialization with Complete Default Values

Always provide complete default values for proper type inference:

```typescript
const form = useForm({
  defaultValues: {
    title: "",
    summary: "",
    mainRepositoryId: "",
    additionalRepositoryIds: [] as string[],
    createWorktree: false,
    worktreeConfigs: {} as Record<string, WorktreeConfig>,
  },
  onSubmit: async ({ value }) => {
    // Handle submission
  },
});
```

---

## Validation Patterns

### Schema Validation with Zod

Integrate Zod schemas for comprehensive validation. Create the schema separately for reusability:

```typescript
import { z } from "zod";

const createIssueSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "Title is required")
    .max(200, "Title must be 200 characters or less"),
  summary: z.string().min(1, "Summary is required"),
  mainRepositoryId: z.string().min(1, "Repository is required"),
  additionalRepositoryIds: z.array(z.string()).optional(),
});
```

### Form-Level Validation with Zod

Apply Zod schema validation at the form level:

```typescript
const form = useForm({
  defaultValues: {
    title: "",
    summary: "",
    mainRepositoryId: "",
  },
  validators: {
    onChange: ({ value }) => {
      const result = createIssueSchema.safeParse(value);
      if (!result.success) {
        const fieldErrors: Record<string, string> = {};
        for (const issue of result.error.issues) {
          if (issue.path.length === 1 && typeof issue.path[0] === "string") {
            fieldErrors[issue.path[0]] = issue.message;
          }
        }
        if (Object.keys(fieldErrors).length > 0) {
          return { fields: fieldErrors };
        }
      }
      return undefined;
    },
    onSubmit: ({ value }) => {
      const result = createIssueSchema.safeParse(value);
      if (!result.success) {
        const fieldErrors: Record<string, string> = {};
        for (const issue of result.error.issues) {
          if (issue.path.length === 1 && typeof issue.path[0] === "string") {
            fieldErrors[issue.path[0]] = issue.message;
          }
        }
        if (Object.keys(fieldErrors).length > 0) {
          return { fields: fieldErrors };
        }
      }
      return undefined;
    },
  },
});
```

### Field-Level Validation

For per-field validation with different timing:

```typescript
<form.Field
  name="email"
  validators={{
    onChange: ({ value }) =>
      !value.includes("@") ? "Invalid email" : undefined,
    onBlur: ({ value }) =>
      value.length === 0 ? "Email is required" : undefined,
  }}
>
  {(field) => (
    <input
      value={field.state.value}
      onBlur={field.handleBlur}
      onChange={(e) => field.handleChange(e.target.value)}
    />
  )}
</form.Field>
```

### Async Validation with Debouncing

Always debounce async validations to prevent excessive requests:

```typescript
<form.Field
  name="username"
  asyncDebounceMs={500}
  validators={{
    onChangeAsync: async ({ value }) => {
      const isAvailable = await checkUsernameAvailability(value);
      return isAvailable ? undefined : "Username already taken";
    },
  }}
/>
```

---

## Field Components

### TextField Component Pattern

Create reusable field components using `useFieldContext`:

```typescript
function TextField({
  label,
  placeholder,
  type = "text",
  required,
  disabled,
  id,
}: {
  label?: string;
  placeholder?: string;
  type?: string;
  required?: boolean;
  disabled?: boolean;
  id?: string;
}) {
  const field = useFieldContext<string>();
  const computedId = id || field.name;

  return (
    <FieldTech>
      {label && (
        <FieldTechLabel htmlFor={computedId} required={required ?? false}>
          {label}
        </FieldTechLabel>
      )}
      <FieldTechControl
        id={computedId}
        name={field.name}
        type={type}
        {...(placeholder !== undefined ? { placeholder } : {})}
        value={field.state.value}
        onChange={(e) => field.handleChange(e.target.value)}
        onBlur={field.handleBlur}
        disabled={disabled ?? field.state.meta.isValidating}
        required={required ?? false}
        aria-invalid={!field.state.meta.isValid}
        aria-describedby={
          !field.state.meta.isValid ? `${field.name}-error` : undefined
        }
      />
      {!field.state.meta.isValid && field.state.meta.errors.length > 0 && (
        <FieldTechError id={`${field.name}-error`} role="alert">
          {field.state.meta.errors.join(", ")}
        </FieldTechError>
      )}
    </FieldTech>
  );
}
```

### SelectField Component Pattern

```typescript
function SelectField({
  label,
  options,
  placeholder,
  required,
  disabled,
}: {
  label?: string;
  options: Array<{ value: string; label: string }>;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
}) {
  const field = useFieldContext<string>();

  return (
    <FieldTech>
      {label && (
        <FieldTechLabel htmlFor={field.name} required={required ?? false}>
          {label}
        </FieldTechLabel>
      )}
      <Select
        value={field.state.value ?? ""}
        onValueChange={(value) => {
          if (value !== null && value !== undefined) {
            field.handleChange(value);
          }
        }}
        disabled={disabled ?? field.state.meta.isValidating}
      >
        <SelectTrigger size="sm" aria-invalid={!field.state.meta.isValid}>
          <SelectValue {...(placeholder !== undefined ? { placeholder } : {})} />
        </SelectTrigger>
        <SelectPopup>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectPopup>
      </Select>
      {!field.state.meta.isValid && field.state.meta.errors.length > 0 && (
        <FieldTechError id={`${field.name}-error`} role="alert">
          {field.state.meta.errors.join(", ")}
        </FieldTechError>
      )}
    </FieldTech>
  );
}
```

### SubmitButton Component Pattern

```typescript
function SubmitButton({
  label,
  disabled,
  className,
}: {
  label: string;
  disabled?: boolean;
  className?: string;
}) {
  const form = useFormContext();
  const isSubmitting = useStore(form.store, (state) => state.isSubmitting);
  const isFormValid = useStore(form.store, (state) => state.isFormValid);

  return (
    <ButtonTech
      type="submit"
      variant="solid"
      size="default"
      disabled={disabled || isSubmitting || !isFormValid}
      className={className}
      aria-busy={isSubmitting}
    >
      {isSubmitting ? <Spinner className="size-4" aria-live="polite" /> : label}
    </ButtonTech>
  );
}
```

---

## Form State Management

### Subscribing to Form State

Use `useStore` or `form.Subscribe` for reactive UI without unnecessary re-renders:

```typescript
import { useStore } from "@tanstack/react-form";

// Using useStore hook
const fullname = useStore(form.store, (state) => state.values.fullname);
const isFormValid = useStore(form.store, (state) => state.isFormValid);

// Using Subscribe component
<form.Subscribe selector={(state) => state.values.fullname}>
  {(fullname) => <div>Current name: {fullname}</div>}
</form.Subscribe>;
```

### Programmatic Field Value Updates

Use `setFieldValue` for programmatic updates:

```typescript
// Update a field value
form.setFieldValue("branchName", newBranchName, { dontUpdateMeta: true });

// Update with meta (triggers validation, etc.)
form.setFieldValue("createWorktree", true);
```

### Form Reset

Reset the form and optionally set specific field values:

```typescript
const handleReset = useCallback(() => {
  form.reset();
  // Set specific field values after reset if needed
  form.setFieldValue("createWorktree", gitSettings.createWorktreeByDefault, {
    dontUpdateMeta: true,
  });
  // Reset mutation state
  createIssueMutation.reset();
}, [gitSettings.createWorktreeByDefault, createIssueMutation]);
```

---

## Array Fields

### Dynamic Array Fields

Handle array fields with proper methods:

```typescript
<form.Field name="people" mode="array">
  {(field) => (
    <div>
      {field.state.value.map((_, i) => (
        <form.Field key={i} name={`people[${i}].name`}>
          {(subField) => (
            <input
              value={subField.state.value}
              onChange={(e) => subField.handleChange(e.target.value)}
            />
          )}
        </form.Field>
      ))}
      <button onClick={() => field.pushValue({ name: "", age: 0 })} type="button">
        Add person
      </button>
    </div>
  )}
</form.Field>
```

### Large Arrays (100+ items)

For large arrays, use a map structure with an order array instead of direct array indexing for better performance.

---

## Error Handling

### Display Field Errors with Accessibility

```typescript
<form.Field name="email">
  {(field) => (
    <>
      <input
        id={field.name}
        value={field.state.value}
        onChange={(e) => field.handleChange(e.target.value)}
        aria-invalid={!field.state.meta.isValid}
        aria-describedby={!field.state.meta.isValid ? `${field.name}-error` : undefined}
      />
      {!field.state.meta.isValid && field.state.meta.errors.length > 0 && (
        <FieldErrorList errors={field.state.meta.errors} />
      )}
    </>
  )}
</form.Field>
```

### Using FieldErrorList Component

The codebase provides a `FieldErrorList` component for consistent error display:

```typescript
import { FieldErrorList } from "@compozy/ui";

<form.Field name="name">
  {(field) => (
    <Field>
      <FieldLabel htmlFor={field.name}>Organization Name</FieldLabel>
      <Input
        id={field.name}
        value={field.state.value}
        onChange={(e) => field.handleChange(e.target.value)}
      />
      <FieldErrorList errors={field.state.meta.errors} />
    </Field>
  )}
</form.Field>;
```

### Mutation Error Display

Display mutation errors separately from field errors:

```typescript
{
  createOrganization.error && (
    <div className="rounded-md border border-destructive bg-destructive/10 p-3" role="alert">
      <p className="text-sm text-destructive">
        {createOrganization.error instanceof Error
          ? createOrganization.error.message
          : "Failed to create organization"}
      </p>
    </div>
  );
}
```

---

## Performance Optimization

### Prefer `onBlur` Validation

Only use `onChange` validation when immediate feedback is required. For most fields, `onBlur` provides a better user experience:

```typescript
validators: {
  onBlur: ({ value }) =>
    value.length === 0 ? "Field is required" : undefined,
}
```

### Subscribe to Specific State

Instead of subscribing to the entire form state, subscribe to specific values:

```typescript
// GOOD: Subscribe to specific values
const isSubmitting = useStore(form.store, (state) => state.isSubmitting);

// BAD: Don't subscribe to entire state
const formState = useStore(form.store, (state) => state);
```

### Side Effects Without Re-renders

For side effects that don't need to trigger re-renders, subscribe directly to `form.store`:

```typescript
useEffect(() => {
  const unsubscribe = form.store.subscribe((state) => {
    // Side effect logic here
  });
  return unsubscribe;
}, []);
```

---

## Anti-Patterns to Avoid

### Never Do This

1. **Don't manually specify generics when defaultValues provides type inference**

```typescript
// BAD
const form = useForm<FormValues>({ ... });

// GOOD
const form = useForm({ defaultValues: { ... } });
```

2. **Don't forget to prevent default on form submission**

```typescript
// GOOD
<form
  onSubmit={(e) => {
    e.preventDefault();
    e.stopPropagation();
    void form.handleSubmit();
  }}
>
```

3. **Don't use onChange validation when onBlur would suffice**

4. **Don't subscribe to the entire form state when you only need specific values**

5. **Don't forget asyncDebounceMs for async validation**

```typescript
// BAD
<form.Field
  name="username"
  validators={{
    onChangeAsync: async ({ value }) => { ... },
  }}
/>

// GOOD
<form.Field
  name="username"
  asyncDebounceMs={500}
  validators={{
    onChangeAsync: async ({ value }) => { ... },
  }}
/>
```

6. **Don't forget accessibility attributes**

```typescript
// GOOD
<input
  aria-invalid={!field.state.meta.isValid}
  aria-describedby={!field.state.meta.isValid ? `${field.name}-error` : undefined}
/>
<span id={`${field.name}-error`} role="alert">
  {field.state.meta.errors.join(", ")}
</span>
```

---

## Validation Checklist

Before finishing a task involving TanStack Form:

- [ ] Use `createFormHook` with `useAppForm` instead of raw `useForm` for consistency (when applicable)
- [ ] Provide complete default values for proper type inference
- [ ] Use Zod schemas for validation when possible
- [ ] Debounce async validations (minimum 500ms recommended)
- [ ] Prevent default on form submission
- [ ] Display errors with proper accessibility (`role="alert"`, `aria-invalid`, `aria-describedby`)
- [ ] Use `onBlur` validation over `onChange` when immediate feedback isn't needed
- [ ] Run type checks (`pnpm run typecheck`) and tests (`pnpm run test`)
