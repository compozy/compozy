# TanStack Form Patterns

## Form Hook Setup with `createFormHook`

Use `createFormHook` + `createFormHookContexts` to build a custom form hook with pre-registered field/form components, instead of re-wiring context and boilerplate per form:

```tsx
import { createFormHook, createFormHookContexts } from '@tanstack/react-form'

export const { fieldContext, formContext, useFieldContext, useFormContext } =
  createFormHookContexts()

export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField, SelectField },
  formComponents: { SubmitButton },
})
```

Provide complete `defaultValues` so TanStack Form infers types from them — don't manually specify generics:

```tsx
const form = useAppForm({
  defaultValues: {
    title: '',
    summary: '',
    mainRepositoryId: '',
    additionalRepositoryIds: [] as string[],
    createWorktree: false,
  },
  onSubmit: async ({ value }) => {
    // Handle submission
  },
})
```

Drive submission through `handleSubmit` and call `e.preventDefault()` (plus `stopPropagation()`) so the browser doesn't navigate:

```tsx
<form
  onSubmit={e => {
    e.preventDefault()
    e.stopPropagation()
    void form.handleSubmit()
  }}
>
```

`withForm` composes shared field sets across multiple forms.

## Validation with Zod

Define the schema once for reuse, then apply it at the form level via `validators.onChange`/`onSubmit`, mapping Zod issues to per-field errors:

```tsx
import { z } from 'zod'

const createIssueSchema = z.object({
  title: z.string().trim().min(1, 'Title is required').max(200),
  summary: z.string().min(1, 'Summary is required'),
  mainRepositoryId: z.string().min(1, 'Repository is required'),
  additionalRepositoryIds: z.array(z.string()).optional(),
})

function zodFieldErrors(result: z.SafeParseError<unknown>) {
  const fieldErrors: Record<string, string> = {}
  for (const issue of result.error.issues) {
    if (issue.path.length === 1 && typeof issue.path[0] === 'string') {
      fieldErrors[issue.path[0]] = issue.message
    }
  }
  return Object.keys(fieldErrors).length > 0 ? { fields: fieldErrors } : undefined
}

const form = useAppForm({
  defaultValues: { title: '', summary: '', mainRepositoryId: '' },
  validators: {
    onChange: ({ value }) => {
      const result = createIssueSchema.safeParse(value)
      return result.success ? undefined : zodFieldErrors(result)
    },
    onSubmit: ({ value }) => {
      const result = createIssueSchema.safeParse(value)
      return result.success ? undefined : zodFieldErrors(result)
    },
  },
})
```

Field-level validators run per-field with their own timing; prefer `onBlur` over `onChange` when immediate feedback isn't required:

```tsx
<form.Field
  name="email"
  validators={{
    onChange: ({ value }) => (!value.includes('@') ? 'Invalid email' : undefined),
    onBlur: ({ value }) => (value.length === 0 ? 'Email is required' : undefined),
  }}
>
  {(field) => (
    <input
      value={field.state.value}
      onBlur={field.handleBlur}
      onChange={e => field.handleChange(e.target.value)}
    />
  )}
</form.Field>
```

## Field Components

Build reusable field components with `useFieldContext`, wiring `value`/`handleChange`/`handleBlur` and exposing errors accessibly (`aria-invalid`, `aria-describedby`, `role="alert"`):

```tsx
function TextField({
  label,
  placeholder,
  type = 'text',
  required,
  disabled,
  id,
}: {
  label?: string
  placeholder?: string
  type?: string
  required?: boolean
  disabled?: boolean
  id?: string
}) {
  const field = useFieldContext<string>()
  const computedId = id || field.name

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
        onChange={e => field.handleChange(e.target.value)}
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
          {field.state.meta.errors.join(', ')}
        </FieldTechError>
      )}
    </FieldTech>
  )
}
```

Form-level components read shared form state via `useFormContext` + `useStore` — e.g. a submit button that disables while `isSubmitting` or the form is invalid:

```tsx
function SubmitButton({ label, disabled, className }: { label: string; disabled?: boolean; className?: string }) {
  const form = useFormContext()
  const isSubmitting = useStore(form.store, state => state.isSubmitting)
  const isFormValid = useStore(form.store, state => state.isFormValid)

  return (
    <ButtonTech
      type="submit"
      variant="solid"
      disabled={disabled || isSubmitting || !isFormValid}
      className={className}
      aria-busy={isSubmitting}
    >
      {isSubmitting ? <Spinner className="size-4" aria-live="polite" /> : label}
    </ButtonTech>
  )
}
```

## Async Validation with Debouncing

Always debounce async validators with `asyncDebounceMs` (≥500ms) so each keystroke doesn't fire a request — an un-debounced `onChangeAsync` hammers the server:

```tsx
<form.Field
  name="username"
  asyncDebounceMs={500}
  validators={{
    onChangeAsync: async ({ value }) => {
      const isAvailable = await checkUsernameAvailability(value)
      return isAvailable ? undefined : 'Username already taken'
    },
  }}
/>
```

## Array Fields

Handle dynamic collections with `mode="array"`, indexing sub-fields by position and mutating via helpers like `pushValue`:

```tsx
<form.Field name="people" mode="array">
  {(field) => (
    <div>
      {field.state.value.map((_, i) => (
        <form.Field key={i} name={`people[${i}].name`}>
          {(subField) => (
            <input
              value={subField.state.value}
              onChange={e => subField.handleChange(e.target.value)}
            />
          )}
        </form.Field>
      ))}
      <button onClick={() => field.pushValue({ name: '', age: 0 })} type="button">
        Add person
      </button>
    </div>
  )}
</form.Field>
```

For large arrays (100+ items), use a map keyed by id with a separate order array instead of direct index access for better performance.

## Error Handling and Accessibility

Give every field `aria-invalid` and `aria-describedby`, and render its error in a `role="alert"` region so screen readers announce it:

```tsx
<form.Field name="email">
  {(field) => (
    <>
      <input
        id={field.name}
        value={field.state.value}
        onChange={e => field.handleChange(e.target.value)}
        aria-invalid={!field.state.meta.isValid}
        aria-describedby={
          !field.state.meta.isValid ? `${field.name}-error` : undefined
        }
      />
      {!field.state.meta.isValid && field.state.meta.errors.length > 0 && (
        <span id={`${field.name}-error`} role="alert">
          {field.state.meta.errors.join(', ')}
        </span>
      )}
    </>
  )}
</form.Field>
```

Display mutation/API errors separately from field validation errors:

```tsx
{createOrganization.error && (
  <div className="rounded-md border border-destructive bg-destructive/10 p-3" role="alert">
    <p className="text-sm text-destructive">
      {createOrganization.error instanceof Error
        ? createOrganization.error.message
        : 'Failed to create organization'}
    </p>
  </div>
)}
```
