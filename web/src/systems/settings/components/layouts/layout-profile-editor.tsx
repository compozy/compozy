import { Button, Field, FieldDescription, FieldLabel, Input, PillGroup, cn } from "@compozy/ui";

import type { WindowManagerLayoutProfilesModel } from "../../hooks/use-window-manager-layout-profiles";
import type {
  WindowManagerLayoutAspect,
  WindowManagerLayoutDocument,
  WindowManagerLayoutOverflow,
  WindowManagerLayoutScopeKind,
} from "../../lib/window-manager-layout-types";

interface LayoutProfileEditorProps {
  editor: WindowManagerLayoutProfilesModel;
  document: WindowManagerLayoutDocument;
  onClose: () => void;
}

const SCOPES: ReadonlyArray<{ value: WindowManagerLayoutScopeKind; label: string; hint: string }> =
  [
    {
      value: "workspace",
      label: "This project",
      hint: "Visible only inside this project.",
    },
    {
      value: "global",
      label: "Every project",
      hint: "Available in every project on this machine.",
    },
  ];

const ASPECTS: ReadonlyArray<{ value: WindowManagerLayoutAspect; label: string }> = [
  { value: "any", label: "Any" },
  { value: "landscape", label: "Landscape" },
  { value: "portrait", label: "Portrait" },
];

const OVERFLOWS: ReadonlyArray<{
  value: WindowManagerLayoutOverflow;
  label: string;
  hint: string;
}> = [
  {
    value: "stack",
    label: "Fold into a stack",
    hint: "Windows that no longer fit share one tile.",
  },
  {
    value: "reject",
    label: "Refuse to restore",
    hint: "The layout is not applied at all.",
  },
];

/** The five fields a `window_layout` resource actually stores, and nothing else. */
export function LayoutProfileEditor({ editor, document, onClose }: LayoutProfileEditorProps) {
  const slots = Object.keys(document.windows).length;
  const forking = editor.selected !== null && editor.selected.id !== editor.id.trim();
  const scopeHint = SCOPES.find(scope => scope.value === editor.scope)?.hint;

  return (
    <div
      className="flex flex-col overflow-hidden rounded-lg bg-canvas shadow-card"
      data-testid="layout-profile-editor"
    >
      <div className="grid gap-3.5 p-4 sm:grid-cols-2">
        <ProfileField htmlFor="layout-profile-name" label="Name">
          <Input
            className="h-8"
            id="layout-profile-name"
            placeholder="Two-up review"
            value={editor.displayName}
            onChange={event => editor.setDisplayName(event.target.value)}
          />
        </ProfileField>
        <ProfileField
          hint={
            forking
              ? "Changing the id saves a new layout instead of replacing this one."
              : "Unique among these layouts. A project layout replaces an everywhere layout with the same ID."
          }
          htmlFor="layout-profile-id"
          label="Resource ID"
        >
          <Input
            className="h-8 font-mono"
            id="layout-profile-id"
            placeholder="two-up-review"
            value={editor.id}
            onChange={event => editor.setId(event.target.value)}
          />
        </ProfileField>
        <ProfileField hint={scopeHint} label="Who can use it">
          <PillGroup
            aria-label="Who can use it"
            items={SCOPES}
            value={editor.scope}
            onChange={editor.setScope}
            size="sm"
          />
        </ProfileField>
        <ProfileField
          hint="Stored on the layout. Nothing selects a layout by shape yet."
          label="Screen shape"
        >
          <PillGroup
            aria-label="Screen shape"
            items={ASPECTS}
            value={editor.aspect}
            onChange={editor.setAspect}
            size="sm"
          />
        </ProfileField>
        <ProfileField className="sm:col-span-2" label="When there is not enough room">
          <PillGroup
            aria-label="When there is not enough room"
            items={OVERFLOWS}
            value={editor.overflow}
            onChange={editor.setOverflow}
            size="sm"
          />
        </ProfileField>
      </div>
      <div className="flex flex-wrap items-center gap-2 border-t border-line-soft px-4 py-2.5">
        <span className="flex-1 text-form-hint text-subtle">
          {slots} window{slots === 1 ? "" : "s"} captured from the layout in the editor.
        </span>
        {editor.error ? (
          <span className="text-form-hint text-danger" role="alert">
            {editor.error.message}
          </span>
        ) : null}
        <Button size="sm" type="button" variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button
          data-testid="layout-profile-save"
          disabled={
            editor.id.trim() === "" || editor.displayName.trim() === "" || editor.phase === "saving"
          }
          size="sm"
          type="button"
          onClick={editor.saveProfile}
        >
          {editor.phase === "saving" ? "Saving…" : "Save layout"}
        </Button>
      </div>
    </div>
  );
}

function ProfileField({
  label,
  hint,
  htmlFor,
  className,
  children,
}: {
  label: string;
  hint?: string;
  htmlFor?: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <Field className={cn("min-w-0 gap-1.5", className)}>
      <FieldLabel className="text-form-label font-medium text-fg" htmlFor={htmlFor}>
        {label}
      </FieldLabel>
      {children}
      {hint ? <FieldDescription className="text-form-hint">{hint}</FieldDescription> : null}
    </Field>
  );
}
