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
  type AgentAvatarAppearance,
  type SymbolValue,
} from "@compozy/ui";

import {
  PROFILE_EMOJIBASE_URL,
  PROFILE_IDENTITY_SWATCHES,
  PROFILE_SPRITE_URL,
  useProfileIconCatalog,
} from "@/systems/profiles";
import { useThemePreference } from "@/systems/theme";

import { nextAgentAvatarSeed } from "../lib/agent-avatar-seed";
import { AgentMark } from "./agent-mark";

const HERO = 112;
const FACE = 92;
const SHUFFLE_PICKS = 6;
const SYMBOL_COLOR = "#2aa8a8";

type Mode = "generated" | "symbol";

const MODE_ITEMS = [
  { value: "generated" as const, label: "Character" },
  { value: "symbol" as const, label: "Icon or emoji" },
];

function seedOf(appearance: AgentAvatarAppearance) {
  return appearance.avatar === "generated" ? appearance.seed : "new-agent";
}

function generated(seed: string, color: string | undefined): AgentAvatarAppearance {
  return { avatar: "generated", seed, generation: 1, ...(color ? { color } : {}) };
}

function CharacterPane({
  appearance,
  name,
  onChange,
}: {
  appearance: AgentAvatarAppearance;
  name: string;
  onChange: (next: AgentAvatarAppearance) => void;
}) {
  const seed = seedOf(appearance);
  const { color } = appearance;
  const picks = Array.from({ length: SHUFFLE_PICKS }, (_, index) => `${seed}.${index}`);
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-3 gap-2">
        {picks.map(pick => (
          <button
            aria-label={`Pick this face for ${name || "your agent"}`}
            className="grid h-20 place-items-center rounded-md bg-sunken outline-none hover:bg-hover focus-visible:shadow-focus-ring"
            key={pick}
            onClick={() => onChange(generated(pick, color))}
            type="button"
          >
            <AgentMark
              appearance={generated(pick, color)}
              label={name || "New agent"}
              quietIdle
              size={52}
            />
          </button>
        ))}
      </div>
      <Button
        onClick={() => onChange(generated(nextAgentAvatarSeed(seed), color))}
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
          onClick={() => onChange(generated(seed, undefined))}
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
            onClick={() => onChange(generated(seed, swatch.value))}
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
  appearance: AgentAvatarAppearance;
  onChange: (next: AgentAvatarAppearance) => void;
}) {
  const { resolvedTheme } = useThemePreference();
  const catalog = useProfileIconCatalog();
  const color = appearance.color ?? SYMBOL_COLOR;
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

/** The big avatar: click to change it, or press the dice to get a different face right away. */
export function AgentAvatarChooser({
  appearance,
  name,
  onChange,
}: {
  appearance: AgentAvatarAppearance;
  name: string;
  onChange: (next: AgentAvatarAppearance) => void;
}) {
  const [mode, setMode] = useState<Mode>(
    appearance.avatar === "generated" ? "generated" : "symbol"
  );
  return (
    <div className="relative shrink-0" style={{ height: HERO, width: HERO }}>
      <Popover>
        <PopoverTrigger
          render={
            <button
              aria-label="Change how your agent looks"
              className="group grid size-full place-items-center rounded-2xl outline-none hover:bg-hover focus-visible:shadow-focus-ring data-popup-open:bg-hover"
              data-testid="agent-create-avatar"
              type="button"
            />
          }
        >
          <AgentMark appearance={appearance} label={name || "New agent"} quietIdle size={FACE} />
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
                onChange(generated(nextAgentAvatarSeed(seedOf(appearance)), appearance.color))
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
