import { Button, Field, FieldDescription, FieldLabel, Input, cn } from "@agh/ui";

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
      label: "This workspace",
      hint: "Visible only inside this workspace.",
    },
    {
      value: "global",
      label: "Every workspace",
      hint: "Available in every workspace on this daemon.",
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
      className="flex flex-col overflow-hidden rounded-lg border border-line bg-canvas-soft"
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
              : "Unique within this scope. A workspace layout may override a global layout with the same id."
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
          <Segmented
            ariaLabel="Who can use it"
            options={SCOPES}
            value={editor.scope}
            onChange={editor.setScope}
          />
        </ProfileField>
        <ProfileField
          hint="Stored on the layout. Nothing selects a layout by shape yet."
          label="Screen shape"
        >
          <Segmented
            ariaLabel="Screen shape"
            options={ASPECTS}
            value={editor.aspect}
            onChange={editor.setAspect}
          />
        </ProfileField>
        <ProfileField className="sm:col-span-2" label="When there is not enough room">
          <Segmented
            ariaLabel="When there is not enough room"
            options={OVERFLOWS}
            value={editor.overflow}
            onChange={editor.setOverflow}
          />
        </ProfileField>
      </div>
      <div className="flex flex-wrap items-center gap-2 border-t border-line-soft bg-canvas-tint px-4 py-2.5">
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
            editor.id.trim() === "" || editor.displayName.trim() === "" || editor.save.isPending
          }
          size="sm"
          type="button"
          onClick={() => editor.save.mutate()}
        >
          {editor.save.isPending ? "Saving…" : "Save layout"}
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

function Segmented<TValue extends string>({
  ariaLabel,
  options,
  value,
  onChange,
}: {
  ariaLabel: string;
  options: ReadonlyArray<{ value: TValue; label: string }>;
  value: TValue;
  onChange: (next: TValue) => void;
}) {
  return (
    <div
      aria-label={ariaLabel}
      className="inline-flex w-fit gap-0.5 rounded-md border border-line-soft bg-canvas-tint p-0.5"
      role="radiogroup"
    >
      {options.map(option => (
        <button
          aria-checked={option.value === value}
          className={cn(
            "inline-flex h-6.5 items-center rounded-sm px-2.5 text-form-label font-medium text-muted",
            "transition-colors duration-base ease-out hover:text-fg",
            "focus-visible:outline-none focus-visible:shadow-focus-ring",
            option.value === value && "bg-elevated text-fg-strong shadow-highlight"
          )}
          key={option.value}
          role="radio"
          type="button"
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
