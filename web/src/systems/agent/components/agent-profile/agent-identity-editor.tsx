/** S5: who the agent is, edited where it's shown. The avatar opens its picker, the words edit in place. */
import { Dices, Pencil } from "@compozy/ui/icons";
import { useState, type KeyboardEvent } from "react";

import {
  Button,
  FieldError,
  Input,
  PillGroup,
  Popover,
  PopoverContent,
  PopoverTrigger,
  SymbolPicker,
  Textarea,
  cn,
  identitySurfaceFor,
  type AgentAvatarAppearance,
  type AgentAvatarState,
  type SymbolValue,
} from "@compozy/ui";

import {
  PROFILE_EMOJIBASE_URL,
  PROFILE_IDENTITY_SWATCHES,
  PROFILE_SPRITE_URL,
  useProfileIconCatalog,
} from "@/systems/profiles";
import { useThemePreference } from "@/systems/theme";

import { nextAgentAvatarSeed } from "../../lib/agent-avatar-seed";
import { AgentMark } from "../agent-mark";

type AvatarMode = "generated" | "symbol";

const MODE_ITEMS = [
  { value: "generated" as const, label: "Generated" },
  { value: "symbol" as const, label: "Icon or emoji" },
];

const SYMBOL_COLOR = "#2aa8a8";

interface PaneProps {
  appearance: AgentAvatarAppearance;
  name: string;
  label: string;
  onPick: (next: AgentAvatarAppearance) => void;
}

function generated(seed: string, color: string | undefined): AgentAvatarAppearance {
  return { avatar: "generated", seed, generation: 1, ...(color ? { color } : {}) };
}

function GeneratedPane({ appearance, name, label, onPick }: PaneProps) {
  const seed = appearance.avatar === "generated" ? appearance.seed : name;
  const { color } = appearance;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3 rounded-md bg-sunken p-3">
        <AgentMark
          appearance={generated(seed, color)}
          label={label}
          quietIdle
          size={48}
          surfaceClassName="ring-sunken"
        />
        <Button
          onClick={() => onPick(generated(nextAgentAvatarSeed(seed), color))}
          size="sm"
          type="button"
          variant="secondary"
        >
          <Dices aria-hidden="true" />
          Shuffle
        </Button>
      </div>
      <fieldset className="flex items-center gap-1.5">
        <legend className="sr-only">Color</legend>
        <Button
          aria-pressed={color === undefined}
          onClick={() => onPick(generated(seed, undefined))}
          size="xs"
          type="button"
          variant={color === undefined ? "secondary" : "ghost"}
        >
          Auto
        </Button>
        {PROFILE_IDENTITY_SWATCHES.map(swatch => (
          <button
            aria-label={swatch.label}
            aria-pressed={color === swatch.value}
            className={cn(
              "size-5 rounded-full outline-none focus-visible:shadow-focus-ring",
              color === swatch.value && "ring-2 ring-fg ring-offset-2 ring-offset-popover"
            )}
            key={swatch.value}
            onClick={() => onPick(generated(seed, swatch.value))}
            style={{ backgroundColor: swatch.value }}
            type="button"
          />
        ))}
      </fieldset>
    </div>
  );
}

function SymbolPane({ appearance, onPick }: Pick<PaneProps, "appearance" | "onPick">) {
  const { resolvedTheme } = useThemePreference();
  const catalog = useProfileIconCatalog();
  const color = appearance.color ?? SYMBOL_COLOR;
  const symbol: SymbolValue =
    appearance.avatar === "emoji"
      ? { kind: "emoji", value: appearance.emoji }
      : { kind: "icon", value: appearance.avatar === "icon" ? appearance.icon : "compass" };
  const commit = (next: SymbolValue, nextColor: string) =>
    onPick(
      next.kind === "emoji"
        ? { avatar: "emoji", emoji: next.value, color: nextColor }
        : { avatar: "icon", icon: next.value, color: nextColor }
    );
  return (
    <SymbolPicker
      color={color}
      emojibaseUrl={PROFILE_EMOJIBASE_URL}
      icons={catalog.icons}
      iconsLoading={catalog.loading}
      onColorChange={next => commit(symbol, next)}
      onSymbolChange={next => commit(next, color)}
      spriteUrl={PROFILE_SPRITE_URL}
      surface={identitySurfaceFor(resolvedTheme)}
      swatches={PROFILE_IDENTITY_SWATCHES}
      symbol={symbol}
    />
  );
}

export function AgentAvatarEditor({
  appearance,
  name,
  label,
  provider,
  state,
  size,
  readOnly,
  onSave,
}: {
  appearance: AgentAvatarAppearance;
  name: string;
  label: string;
  provider: string | null;
  state: AgentAvatarState;
  size: number;
  readOnly: boolean;
  onSave: (next: AgentAvatarAppearance) => void;
}) {
  const [picked, setPicked] = useState<AgentAvatarAppearance | null>(null);
  const [mode, setMode] = useState<AvatarMode>(
    appearance.avatar === "generated" ? "generated" : "symbol"
  );
  const current = picked ?? appearance;
  const avatar = (
    <AgentMark
      appearance={current}
      label={label}
      provider={provider}
      quietIdle
      size={size}
      state={state}
    />
  );
  if (readOnly) return avatar;
  return (
    <Popover
      onOpenChange={open => {
        if (open || !picked) return;
        onSave(picked);
        setPicked(null);
      }}
    >
      <PopoverTrigger
        aria-label="Change picture"
        className="group/avatar relative shrink-0 rounded-full outline-none focus-visible:shadow-focus-ring"
        data-testid="agent-profile-avatar"
      >
        {avatar}
        <span
          aria-hidden="true"
          className="absolute bottom-0 left-0 grid size-6 place-items-center rounded-full bg-elevated text-muted-foreground opacity-0 shadow-card transition-opacity duration-base group-hover/avatar:opacity-100 group-focus-visible/avatar:opacity-100 motion-reduce:transition-none"
        >
          <Pencil className="size-3" />
        </span>
      </PopoverTrigger>
      <PopoverContent align="start" className="flex w-96 flex-col gap-3">
        <PillGroup items={MODE_ITEMS} onChange={setMode} size="md" value={mode} />
        {mode === "generated" ? (
          <GeneratedPane appearance={current} label={label} name={name} onPick={setPicked} />
        ) : (
          <SymbolPane appearance={current} onPick={setPicked} />
        )}
      </PopoverContent>
    </Popover>
  );
}

function problemWith(text: string, label: string, max: number, required: boolean) {
  if (required && text.trim() === "") return `${label} can't be empty.`;
  if (text.length > max) return `${label} can have up to ${max} characters.`;
  return null;
}

export interface AgentInlineTextProps {
  /** Names the field for screen readers and in its error. */
  label: string;
  value: string;
  max: number;
  required?: boolean;
  multiline?: boolean;
  placeholder?: string;
  readOnly?: boolean;
  /** Type classes, shared by the words and the field so nothing jumps on click. */
  textClassName: string;
  onSave: (next: string) => void;
}

/** Words that turn into a field where they stand: Enter or leaving saves, Esc puts them back. */
export function AgentInlineText({
  label,
  value,
  max,
  required = false,
  multiline = false,
  placeholder,
  readOnly = false,
  textClassName,
  onSave,
}: AgentInlineTextProps) {
  const [draft, setDraft] = useState<string | null>(null);
  if (readOnly) return <p className={cn("min-w-0", textClassName)}>{value}</p>;
  if (draft === null) {
    return (
      <button
        aria-label={`Edit ${label.toLowerCase()}: ${value || "empty"}`}
        className={cn(
          "group/edit relative isolate flex max-w-full min-w-0 items-center gap-2 self-start text-left outline-none",
          "before:absolute before:-inset-x-1.5 before:inset-y-0 before:-z-10 before:rounded-sm before:transition-colors before:duration-base hover:before:bg-hover focus-visible:before:shadow-focus-ring motion-reduce:before:transition-none",
          textClassName,
          !value && "text-subtle"
        )}
        onClick={() => setDraft(value)}
        type="button"
      >
        <span className="min-w-0">{value || placeholder}</span>
        <Pencil
          aria-hidden="true"
          className="size-3 shrink-0 text-subtle opacity-0 transition-opacity duration-base group-hover/edit:opacity-100 group-focus-visible/edit:opacity-100 motion-reduce:transition-none"
        />
      </button>
    );
  }
  const problem = problemWith(draft, label, max, required);
  const commit = () => {
    if (problem) return;
    const next = draft.trim();
    setDraft(null);
    if (next !== value) onSave(next);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") setDraft(null);
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      commit();
    }
  };
  const shared = {
    "aria-invalid": problem ? true : undefined,
    "aria-label": label,
    autoFocus: true,
    onBlur: commit,
    onChange: (event: { target: { value: string } }) => setDraft(event.target.value),
    onKeyDown,
    placeholder,
    value: draft,
  };
  return (
    <div className="-mx-1.5 flex min-w-0 flex-col gap-1">
      {multiline ? (
        <Textarea {...shared} className={cn("min-h-0 px-1.5 py-1", textClassName)} rows={2} />
      ) : (
        <Input {...shared} className={cn("h-auto px-1.5 py-0.5", textClassName)} />
      )}
      {problem ? <FieldError>{problem}</FieldError> : null}
    </div>
  );
}
