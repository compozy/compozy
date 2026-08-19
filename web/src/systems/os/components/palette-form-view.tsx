import { useRef, useState, type FormEvent } from "react";

import {
  Button,
  Checkbox,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
} from "@compozy/ui";

import type { CmdPaletteViewAction, CmdPaletteViewForm } from "../lib/cmd-palette-types";

type FormValue = string | boolean | readonly string[];

export function PaletteFormView({
  form,
  onSubmit,
}: {
  form: CmdPaletteViewForm;
  onSubmit: (
    action: CmdPaletteViewAction | undefined,
    values: Readonly<Record<string, FormValue>>
  ) => Promise<void>;
}) {
  const [values, setValues] = useState<Record<string, FormValue>>(() => initialValues(form));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLFormElement | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const invalid: Record<string, string> = {};
    for (const field of form.fields) {
      if (field.required && isEmptyValue(values[field.id])) invalid[field.id] = "Required";
    }
    setErrors(invalid);
    const first = form.fields.find(field => invalid[field.id] !== undefined);
    if (first) {
      const fieldContainer = [
        ...(formRef.current?.querySelectorAll<HTMLElement>("[data-field-id]") ?? []),
      ].find(element => element.dataset.fieldId === first.id);
      fieldContainer?.querySelector<HTMLElement>("input, textarea, button")?.focus();
      return;
    }
    setSubmitting(true);
    setSubmitError("");
    try {
      await onSubmit(form.submit ?? undefined, values);
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "The form could not be submitted.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form ref={formRef} className="space-y-4 p-4" data-testid="palette-form-view" onSubmit={submit}>
      {form.fields.map(field => {
        const error = errors[field.id] ?? field.error;
        return (
          <div key={field.id} className="space-y-1.5" data-field-id={field.id}>
            <Label htmlFor={`palette-field-${field.id}`}>{field.label}</Label>
            {renderField(field, values[field.id], value => {
              setValues(current => ({ ...current, [field.id]: value }));
              if (errors[field.id]) {
                setErrors(current => ({ ...current, [field.id]: "" }));
              }
            })}
            {field.type === "dropdown" && field.options?.length === 0 && field.empty_hint ? (
              <p className="text-small-body text-muted">{field.empty_hint}</p>
            ) : null}
            {error ? (
              <p className="text-small-body text-danger" role="alert">
                {error}
              </p>
            ) : null}
          </div>
        );
      })}
      {submitError ? (
        <p className="text-small-body text-danger" role="alert" data-testid="palette-form-error">
          {submitError}
        </p>
      ) : null}
      <Button disabled={submitting} type="submit">
        {submitting ? "Submitting…" : (form.submit?.title ?? "Submit")}
      </Button>
    </form>
  );
}

function renderField(
  field: CmdPaletteViewForm["fields"][number],
  value: FormValue | undefined,
  onChange: (value: FormValue) => void
) {
  const id = `palette-field-${field.id}`;
  if (field.type === "checkbox") {
    return (
      <Checkbox
        id={id}
        checked={value === true}
        onCheckedChange={checked => onChange(checked === true)}
      />
    );
  }
  if (field.type === "dropdown") {
    return (
      <Select
        value={typeof value === "string" ? value : ""}
        onValueChange={next => onChange(next ?? "")}
      >
        <SelectTrigger id={id} className="w-full" aria-invalid={Boolean(field.error)}>
          <SelectValue placeholder={field.placeholder ?? "Select"} />
        </SelectTrigger>
        <SelectContent>
          {field.options?.map(option => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }
  if (field.type === "textarea") {
    return (
      <Textarea
        id={id}
        aria-invalid={Boolean(field.error)}
        placeholder={field.placeholder}
        value={typeof value === "string" ? value : ""}
        onChange={event => onChange(event.target.value)}
      />
    );
  }
  if (field.type === "file") {
    return (
      <Input
        id={id}
        type="file"
        multiple={field.directories}
        onChange={event => onChange([...(event.target.files ?? [])].map(file => file.name))}
      />
    );
  }
  return (
    <Input
      id={id}
      type={field.type === "password" ? "password" : "text"}
      autoComplete={field.type === "password" ? "new-password" : undefined}
      aria-invalid={Boolean(field.error)}
      placeholder={field.placeholder}
      value={typeof value === "string" ? value : ""}
      onChange={event => onChange(event.target.value)}
    />
  );
}

function initialValues(form: CmdPaletteViewForm): Record<string, FormValue> {
  return Object.fromEntries(
    form.fields.map(field => {
      if (field.type === "checkbox") return [field.id, field.default === true];
      return [field.id, typeof field.default === "string" ? field.default : ""];
    })
  );
}

function isEmptyValue(value: FormValue | undefined): boolean {
  return value === undefined || value === "" || (Array.isArray(value) && value.length === 0);
}
