/** The big avatar: click to change it, or press the dice to get a different face right away. */
import { Dices } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Button,
  PillGroup,
  Popover,
  PopoverContent,
  PopoverTrigger,
  SymbolPicker,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
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
import type { AgentAppearance } from "../stays-on-agents-shared/agent-state";
import { useCanvasTheme } from "../stays-on-agents-shared/use-canvas-theme";

import { useIconCatalog } from "./hooks/use-icon-catalog";
import { nextSeed } from "./model";

const HERO = 112;
const SHUFFLE_PICKS = 6;

type Mode = "generated" | "symbol";

const MODE_ITEMS = [
  { value: "generated" as const, label: "Character" },
  { value: "symbol" as const, label: "Icon or emoji" },
];

function seedOf(appearance: AgentAppearance) {
  return appearance.avatar === "generated" ? appearance.seed : "new-agent";
}

function CharacterPane({
  appearance,
  name,
  onChange,
}: {
  appearance: AgentAppearance;
  name: string;
  onChange: (next: AgentAppearance) => void;
}) {
  const seed = seedOf(appearance);
  const color = appearance.color;
  const picks = Array.from({ length: SHUFFLE_PICKS }, (_, index) => `${seed}.${index}`);
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-3 gap-2">
        {picks.map(pick => (
          <button
            aria-label={`Pick this face for ${name || "your agent"}`}
            className="grid h-20 place-items-center rounded-md bg-sunken outline-none hover:bg-hover focus-visible:shadow-focus-ring"
            key={pick}
            onClick={() => onChange({ avatar: "generated", seed: pick, color })}
            type="button"
          >
            <AgentAvatar
              appearance={{ avatar: "generated", seed: pick, color }}
              label={name || "New agent"}
              quietIdle
              size={52}
            />
          </button>
        ))}
      </div>
      <Button
        onClick={() => onChange({ avatar: "generated", seed: nextSeed(seed), color })}
        size="sm"
        type="button"
        variant="secondary"
      >
        <Dices aria-hidden="true" />
        Show me more
      </Button>
      <fieldset className="flex items-center justify-between gap-1">
        <legend className="sr-only">Color</legend>
        <Button
          aria-pressed={color === undefined}
          onClick={() => onChange({ avatar: "generated", seed })}
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
            onClick={() => onChange({ avatar: "generated", seed, color: swatch.value })}
            style={{ backgroundColor: swatch.value }}
            type="button"
          />
        ))}
      </fieldset>
    </div>
  );
}

function SymbolPane({
  appearance,
  onChange,
}: {
  appearance: AgentAppearance;
  onChange: (next: AgentAppearance) => void;
}) {
  const theme = useCanvasTheme();
  const icons = useIconCatalog();
  const color = appearance.color ?? "#2aa8a8";
  const symbol: SymbolValue =
    appearance.avatar === "emoji"
      ? { kind: "emoji", value: appearance.emoji }
      : { kind: "icon", value: appearance.avatar === "icon" ? appearance.icon : "compass" };
  const commit = (next: SymbolValue, nextColor: string) =>
    onChange(
      next.kind === "emoji"
        ? { avatar: "emoji", emoji: next.value, color: nextColor }
        : { avatar: "icon", icon: next.value, color: nextColor }
    );
  return (
    <SymbolPicker
      color={color}
      emojibaseUrl={PROFILE_EMOJIBASE_URL}
      icons={icons}
      iconsLoading={icons.length === 0}
      onColorChange={next => commit(symbol, next)}
      onSymbolChange={next => commit(next, color)}
      spriteUrl={PROFILE_SPRITE_URL}
      surface={identitySurfaceFor(theme)}
      swatches={PROFILE_IDENTITY_SWATCHES}
      symbol={symbol}
    />
  );
}

export interface AvatarChooserProps {
  appearance: AgentAppearance;
  name: string;
  onChange: (next: AgentAppearance) => void;
  defaultOpen?: Mode;
}

export function AvatarChooser({ appearance, name, onChange, defaultOpen }: AvatarChooserProps) {
  const [mode, setMode] = useState<Mode>(
    defaultOpen ?? (appearance.avatar === "generated" ? "generated" : "symbol")
  );
  return (
    <div className="relative shrink-0" style={{ height: HERO, width: HERO }}>
      <Popover defaultOpen={defaultOpen !== undefined}>
        <PopoverTrigger
          render={
            <button
              aria-label="Change how your agent looks"
              className="group grid size-full place-items-center rounded-2xl outline-none hover:bg-hover focus-visible:shadow-focus-ring data-popup-open:bg-hover"
              type="button"
            />
          }
        >
          <AgentAvatar appearance={appearance} label={name || "New agent"} quietIdle size={92} />
        </PopoverTrigger>
        <PopoverContent align="start" className="flex w-80 flex-col gap-4 p-3">
          <PillGroup items={MODE_ITEMS} onChange={setMode} size="md" value={mode} />
          {mode === "generated" ? (
            <CharacterPane appearance={appearance} name={name} onChange={onChange} />
          ) : (
            <SymbolPane appearance={appearance} onChange={onChange} />
          )}
        </PopoverContent>
      </Popover>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label="Shuffle"
              className="absolute right-0 bottom-0 rounded-full bg-surface-2 shadow-card hover:bg-selected"
              onClick={() =>
                onChange({
                  avatar: "generated",
                  seed: nextSeed(seedOf(appearance)),
                  color: appearance.color,
                })
              }
              size="icon-sm"
              type="button"
              variant="quiet"
            >
              <Dices aria-hidden="true" />
            </Button>
          }
        />
        <TooltipContent>Shuffle</TooltipContent>
      </Tooltip>
    </div>
  );
}
