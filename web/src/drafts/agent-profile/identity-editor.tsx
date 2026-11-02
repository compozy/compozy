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
  type SymbolValue,
} from "@compozy/ui";

import {
  PROFILE_EMOJIBASE_URL,
  PROFILE_IDENTITY_SWATCHES,
  PROFILE_SPRITE_URL,
} from "@/systems/profiles/lib/profile-identity";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AgentAppearance, AgentState } from "../stays-on-agents-shared/agent-state";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";
import { useCanvasTheme } from "../stays-on-agents-shared/use-canvas-theme";
import { useIconCatalog } from "./hooks/use-icon-catalog";

export type AvatarMode = "generated" | "symbol";

const MODE_ITEMS = [
  { value: "generated" as const, label: "Generated" },
  { value: "symbol" as const, label: "Icon or emoji" },
];

interface PaneProps {
  onPick: (next: AgentAppearance) => void;
}

function GeneratedPane({
  agent,
  initialColor,
  onPick,
}: PaneProps & { agent: DraftAgent; initialColor?: string }) {
  const [shuffles, setShuffles] = useState(0);
  const [color, setColor] = useState(initialColor);
  const seed = shuffles === 0 ? agent.name : `${agent.name}-${shuffles}`;
  const pick = (nextShuffles: number, nextColor: string | undefined) => {
    setShuffles(nextShuffles);
    setColor(nextColor);
    onPick({ avatar: "generated", seed: `${agent.name}-${nextShuffles}`, color: nextColor });
  };
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3 rounded-md bg-sunken p-3">
        <AgentAvatar
          appearance={{ avatar: "generated", seed, color }}
          label={agent.label}
          quietIdle
          size={48}
          surfaceClassName="ring-sunken"
        />
        <Button
          onClick={() => pick(shuffles + 1, color)}
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
          onClick={() => pick(shuffles, undefined)}
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
            onClick={() => pick(shuffles, swatch.value)}
            style={{ backgroundColor: swatch.value }}
            type="button"
          />
        ))}
      </fieldset>
    </div>
  );
}

function SymbolPane({ onPick }: PaneProps) {
  const theme = useCanvasTheme();
  const icons = useIconCatalog();
  const [symbol, setSymbol] = useState<SymbolValue>({ kind: "icon", value: "compass" });
  const [color, setColor] = useState("#2aa8a8");
  const pick = (nextSymbol: SymbolValue, nextColor: string) => {
    setSymbol(nextSymbol);
    setColor(nextColor);
    onPick(
      nextSymbol.kind === "emoji"
        ? { avatar: "emoji", emoji: nextSymbol.value, color: nextColor }
        : { avatar: "icon", icon: nextSymbol.value, color: nextColor }
    );
  };
  return (
    <SymbolPicker
      color={color}
      emojibaseUrl={PROFILE_EMOJIBASE_URL}
      icons={icons}
      iconsLoading={icons.length === 0}
      onColorChange={next => pick(symbol, next)}
      onSymbolChange={next => pick(next, color)}
      spriteUrl={PROFILE_SPRITE_URL}
      surface={identitySurfaceFor(theme)}
      swatches={PROFILE_IDENTITY_SWATCHES}
      symbol={symbol}
    />
  );
}

function AvatarPopoverBody({
  agent,
  initial,
  initialColor,
  onPick,
}: PaneProps & {
  agent: DraftAgent;
  initial: AvatarMode;
  initialColor?: string;
}) {
  const [mode, setMode] = useState<AvatarMode>(initial);
  return (
    <div className="flex flex-col gap-3">
      <PillGroup items={MODE_ITEMS} onChange={setMode} size="md" value={mode} />
      {mode === "generated" ? (
        <GeneratedPane agent={agent} initialColor={initialColor} onPick={onPick} />
      ) : (
        <SymbolPane onPick={onPick} />
      )}
    </div>
  );
}

export function AvatarEditor({
  agent,
  state,
  size,
  popover,
  popoverColor,
  readOnly = false,
}: {
  agent: DraftAgent;
  state: AgentState;
  size: number;
  popover?: AvatarMode;
  popoverColor?: string;
  readOnly?: boolean;
}) {
  const [picked, setPicked] = useState<AgentAppearance | null>(null);
  const avatar = (
    <AgentAvatar
      appearance={picked ?? agent.appearance}
      label={agent.label}
      provider={agent.provider}
      quietIdle
      size={size}
      state={state}
    />
  );
  if (readOnly) return avatar;
  return (
    <Popover defaultOpen={popover !== undefined}>
      <PopoverTrigger
        aria-label="Change picture"
        className="group/avatar relative shrink-0 rounded-full outline-none focus-visible:shadow-focus-ring"
      >
        {avatar}
        <span
          aria-hidden="true"
          className="absolute bottom-0 left-0 grid size-6 place-items-center rounded-full bg-elevated text-muted-foreground opacity-0 shadow-card transition-opacity duration-base group-hover/avatar:opacity-100 group-focus-visible/avatar:opacity-100 motion-reduce:transition-none"
        >
          <Pencil className="size-3" />
        </span>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-96">
        <AvatarPopoverBody
          agent={agent}
          initial={popover ?? "generated"}
          initialColor={popoverColor}
          onPick={setPicked}
        />
      </PopoverContent>
    </Popover>
  );
}

function problemWith(text: string, label: string, max: number, required: boolean) {
  if (required && text.trim() === "") return `${label} can't be empty.`;
  if (text.length > max) return `${label} can have up to ${max} characters.`;
  return null;
}

export interface InlineTextProps {
  /** Names the field for screen readers and in its error. */
  label: string;
  value: string;
  /** Text the field opens with, when it differs from the saved value. */
  draft?: string;
  max: number;
  required?: boolean;
  multiline?: boolean;
  placeholder?: string;
  editing?: boolean;
  readOnly?: boolean;
  /** Type classes, shared by the words and the field so nothing jumps on click. */
  textClassName: string;
}

/** Words that turn into a field where they stand: Enter or leaving saves, Esc puts them back. */
export function InlineText({
  label,
  value: initial,
  draft: initialDraft,
  max,
  required = false,
  multiline = false,
  placeholder,
  editing = false,
  readOnly = false,
  textClassName,
}: InlineTextProps) {
  const [value, setValue] = useState(initial);
  const [draft, setDraft] = useState<string | null>(editing ? (initialDraft ?? initial) : null);
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
    setValue(draft.trim());
    setDraft(null);
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
