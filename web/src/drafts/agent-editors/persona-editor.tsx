/** S6: SOUL as a form, using the field names `internal/soul` accepts. */
import { CircleAlert, Plus, X } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  Field,
  FieldError,
  FieldLabel,
  Input,
  Pill,
  PillGroup,
  Textarea,
} from "@compozy/ui";

import { EditorCard } from "../persistent-agents-shared/rich-controls";

const SOUL = `---
version: 1
role: Keeps this project's issues moving
tone: direct
principles:
  - Name who can unblock the work
  - Stay quiet when nothing changed
constraints:
  - Never merge on your own
tags: [triage, issues]
---`;

const INVALID_SOUL = `---
version: 1
role: Keeps this project's issues moving
tone: [direct
---`;

const FREE_FORM_SOUL = `---
version: 1
role: Keeps this project's issues moving
---

# Notes for myself

Long free-form section the form has no field for. It stays exactly as written.`;

type Mode = "form" | "advanced";

const MODES = [
  { value: "form" as const, label: "Form" },
  { value: "advanced" as const, label: "Advanced" },
];

function ListField({ label, values }: { label: string; values: readonly string[] }) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <ul className="flex flex-col gap-1">
        {values.map(value => (
          <li className="flex items-center gap-1" key={value}>
            <Input defaultValue={value} aria-label={label} />
            <Button aria-label={`Remove ${value}`} size="icon-sm" type="button" variant="quiet">
              <X aria-hidden="true" />
            </Button>
          </li>
        ))}
      </ul>
      <Button className="w-fit" size="sm" type="button" variant="ghost">
        <Plus aria-hidden="true" />
        Add
      </Button>
    </Field>
  );
}

function PersonaForm() {
  return (
    <EditorCard>
      <Field>
        <FieldLabel htmlFor="soul-role">Role</FieldLabel>
        <Input defaultValue="Keeps this project's issues moving" id="soul-role" />
      </Field>
      <Field>
        <FieldLabel>Tone</FieldLabel>
        <PillGroup
          items={[
            { value: "direct", label: "Direct" },
            { value: "warm", label: "Warm" },
            { value: "formal", label: "Formal" },
          ]}
          onChange={() => undefined}
          value="direct"
        />
      </Field>
      <ListField
        label="Principles"
        values={["Name who can unblock the work", "Stay quiet when nothing changed"]}
      />
      <ListField label="Constraints" values={["Never merge on your own"]} />
      <Field>
        <FieldLabel htmlFor="soul-collaboration">Collaboration</FieldLabel>
        <Textarea
          defaultValue="Hand implementation to Code Engineer and review to Reviewer."
          id="soul-collaboration"
          rows={2}
        />
      </Field>
      <Field>
        <FieldLabel>Tags</FieldLabel>
        <div className="flex flex-wrap gap-1.5">
          <Pill size="sm">triage</Pill>
          <Pill size="sm">issues</Pill>
        </div>
      </Field>
    </EditorCard>
  );
}

function RawEditor({ value, error }: { value: string; error?: string }) {
  return (
    <Field data-invalid={error ? true : undefined}>
      <Textarea
        aria-invalid={error ? true : undefined}
        aria-label="SOUL.md"
        className="min-h-96 font-mono"
        defaultValue={value}
      />
      {error ? <FieldError>{error}</FieldError> : null}
    </Field>
  );
}

export type PersonaState = "form" | "advanced" | "conflict" | "inexpressible" | "invalid";

export function PersonaEditor({ state }: { state: PersonaState }) {
  const forced = state === "inexpressible" || state === "invalid";
  const [mode, setMode] = useState<Mode>(state === "advanced" || forced ? "advanced" : "form");
  return (
    <div className="flex flex-col gap-4 py-2">
      {forced ? null : <PillGroup items={MODES} onChange={setMode} value={mode} />}
      {state === "conflict" ? (
        <Alert variant="warning">
          <CircleAlert aria-hidden="true" className="size-4" />
          <AlertTitle>Triage Lead changed this file while you were editing</AlertTitle>
          <AlertDescription>
            Your edits are still here. Choose which version to keep.
          </AlertDescription>
          <AlertActions>
            <Button size="xs" type="button" variant="secondary">
              Use theirs
            </Button>
            <Button size="xs" type="button" variant="secondary">
              Keep mine
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
      {state === "inexpressible" ? (
        <Alert variant="info">
          <AlertDescription>
            SOUL.md has a section the form can't show, so it opens here to keep it intact.
          </AlertDescription>
        </Alert>
      ) : null}
      {mode === "form" ? (
        <PersonaForm />
      ) : (
        <RawEditor
          error={
            state === "invalid"
              ? "Line 4: tone has an unclosed list. The file wasn't saved."
              : undefined
          }
          value={
            state === "invalid" ? INVALID_SOUL : state === "inexpressible" ? FREE_FORM_SOUL : SOUL
          }
        />
      )}
    </div>
  );
}
