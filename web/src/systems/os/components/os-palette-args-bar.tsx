import { type KeyboardEvent } from "react";

import { Checkbox, Input, KindIcon, cn } from "@compozy/ui";

import type { PaletteArgField, PaletteArgsState } from "../lib/cmd-palette-args";
import {
  CMD_PALETTE_ICON_FALLBACK,
  cmdPaletteIconRegistry,
  isEmojiIcon,
} from "../lib/cmd-palette-icons";
import { PaletteArgDropdown } from "./os-palette-arg-dropdown";

/** The query box's own grammar — entering arguments is the same surface, narrowed. */
const FIELD_CLASS =
  "h-10 rounded-md border border-line bg-canvas-tint px-2.5 text-small-body text-fg placeholder:text-subtle focus-visible:border-line-strong focus-visible:shadow-focus-ring";

interface PaletteArgFieldRowProps {
  field: PaletteArgField;
  focused: boolean;
  /** Hands the field node up so a blocked submit can focus it. */
  registerNode: (node: HTMLElement | null) => void;
  onChange: (name: string, value: string) => void;
  onSubmit: () => void;
}

function PaletteArgFieldRow({
  field,
  focused,
  registerNode,
  onChange,
  onSubmit,
}: PaletteArgFieldRowProps) {
  return (
    <div className="flex min-w-[8rem] flex-1 flex-col gap-1">
      <label className="text-form-label text-subtle" htmlFor={`os-palette-arg-${field.name}`}>
        {field.name}
      </label>
      {field.type === "dropdown" ? (
        <PaletteArgDropdown
          className={FIELD_CLASS}
          field={field}
          focused={focused}
          registerNode={registerNode}
          onChange={value => onChange(field.name, value)}
          onSubmit={onSubmit}
        />
      ) : field.type === "checkbox" ? (
        <Checkbox
          ref={registerNode}
          aria-describedby={field.error === "" ? undefined : `os-palette-arg-error-${field.name}`}
          aria-invalid={field.error !== "" ? true : undefined}
          autoFocus={focused}
          checked={["true", "yes", "1", "on"].includes(field.value.trim().toLowerCase())}
          className={cn(field.error !== "" && "border-danger")}
          data-testid={`os-palette-arg-${field.name}`}
          id={`os-palette-arg-${field.name}`}
          onCheckedChange={checked => onChange(field.name, checked ? "true" : "false")}
        />
      ) : (
        <Input
          ref={registerNode}
          aria-describedby={field.error === "" ? undefined : `os-palette-arg-error-${field.name}`}
          aria-invalid={field.error !== "" ? true : undefined}
          autoFocus={focused}
          className={cn(FIELD_CLASS, field.error !== "" && "border-danger")}
          data-testid={`os-palette-arg-${field.name}`}
          id={`os-palette-arg-${field.name}`}
          placeholder={field.placeholder}
          // One masking mechanism: a real password input keeps password managers
          // and screen readers correct, and keeps the value out of any echo.
          type={field.type === "password" ? "password" : "text"}
          value={field.value}
          {...(field.type === "password" ? { autoComplete: "off", spellCheck: false } : {})}
          onChange={event => onChange(field.name, event.target.value)}
        />
      )}
      {field.error === "" ? null : (
        <span
          className="text-small-body text-danger"
          data-testid={`os-palette-arg-error-${field.name}`}
          id={`os-palette-arg-error-${field.name}`}
        >
          {field.error}
        </span>
      )}
    </div>
  );
}

export interface PaletteArgsBarProps {
  state: PaletteArgsState;
  onChange: (name: string, value: string) => void;
  /** Returns the field that blocked the submit, or `null` when it went through. */
  onSubmit: () => string | null;
}

/**
 * The input bar as typed argument fields (`_uiux.md` S8, US-015).
 *
 * The head keeps the command's identity on the same rail the breadcrumb uses, so
 * entering arguments reads as this surface narrowing rather than a form
 * appearing on top of it. ⇥ traverses natively — the fields are siblings in
 * declared order — and ⏎ submits from anywhere in the bar, blocking on the first
 * field that is not ready.
 */
export function PaletteArgsBar({ state, onChange, onSubmit }: PaletteArgsBarProps) {
  const emoji = isEmojiIcon(state.icon);
  const fieldNodes = new Map<string, HTMLElement>();
  const firstField = state.fields[0]?.name ?? null;
  /*
   * A blocked submit moves focus to the field that stopped it, every time —
   * including the second time the same field blocks. `autoFocus` only fires on
   * mount, so it opens the bar and the submit handler owns it from there.
   */
  const submit = () => {
    const blocked = onSubmit();
    if (blocked !== null) fieldNodes.get(blocked)?.focus();
  };
  return (
    <div className="flex flex-col gap-2.5 px-2 pt-3 pb-2.5" data-testid="os-palette-args">
      <div className="flex items-center gap-2 px-1 text-small-body text-fg-strong">
        {emoji ? (
          <span aria-hidden="true" className="text-badge leading-none">
            {state.icon}
          </span>
        ) : (
          <KindIcon
            className="size-4 shrink-0 text-subtle"
            fallback={CMD_PALETTE_ICON_FALLBACK}
            kind={state.icon}
            registry={cmdPaletteIconRegistry}
          />
        )}
        <span className="min-w-0 truncate">{state.title}</span>
      </div>
      <div
        className="flex flex-wrap items-start gap-2"
        onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => {
          if (event.key !== "Enter" || event.defaultPrevented) return;
          event.preventDefault();
          submit();
        }}
      >
        {state.fields.map(field => (
          <PaletteArgFieldRow
            field={field}
            focused={field.name === firstField}
            key={field.name}
            registerNode={node => {
              if (node === null) fieldNodes.delete(field.name);
              else fieldNodes.set(field.name, node);
            }}
            onChange={onChange}
            onSubmit={submit}
          />
        ))}
      </div>
    </div>
  );
}
