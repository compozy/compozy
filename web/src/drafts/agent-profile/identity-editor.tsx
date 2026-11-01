/** S5: the avatar tile, popover and identity fields. */
import { Dices } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Field,
  FieldError,
  FieldLabel,
  ImmutableIdentity,
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

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { DraftAgent } from "../persistent-agents-shared/fixtures";
import { useCanvasTheme } from "../persistent-agents-shared/use-canvas-theme";
import { useIconCatalog } from "./hooks/use-icon-catalog";

type Mode = "generated" | "symbol";

const MODE_ITEMS = [
  { value: "generated" as const, label: "Generated" },
  { value: "symbol" as const, label: "Icon or emoji" },
];

function GeneratedPane({ agent, initialColor }: { agent: DraftAgent; initialColor?: string }) {
  const [shuffles, setShuffles] = useState(0);
  const [color, setColor] = useState(initialColor);
  const seed = shuffles === 0 ? agent.name : `${agent.name}-${shuffles}`;
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
          onClick={() => setShuffles(count => count + 1)}
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
          onClick={() => setColor(undefined)}
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
            onClick={() => setColor(swatch.value)}
            style={{ backgroundColor: swatch.value }}
            type="button"
          />
        ))}
      </fieldset>
    </div>
  );
}

function SymbolPane() {
  const theme = useCanvasTheme();
  const icons = useIconCatalog();
  const [symbol, setSymbol] = useState<SymbolValue>({ kind: "icon", value: "compass" });
  const [color, setColor] = useState("#2aa8a8");
  return (
    <SymbolPicker
      color={color}
      emojibaseUrl={PROFILE_EMOJIBASE_URL}
      icons={icons}
      iconsLoading={icons.length === 0}
      onColorChange={setColor}
      onSymbolChange={setSymbol}
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
}: {
  agent: DraftAgent;
  initial: Mode;
  initialColor?: string;
}) {
  const [mode, setMode] = useState<Mode>(initial);
  return (
    <div className="flex flex-col gap-3">
      <PillGroup items={MODE_ITEMS} onChange={setMode} size="md" value={mode} />
      {mode === "generated" ? (
        <GeneratedPane agent={agent} initialColor={initialColor} />
      ) : (
        <SymbolPane />
      )}
    </div>
  );
}

export function IdentityEditor({
  agent,
  popover,
  popoverColor,
  invalid = false,
  readOnly = false,
}: {
  agent: DraftAgent;
  popover?: Mode;
  popoverColor?: string;
  invalid?: boolean;
  readOnly?: boolean;
}) {
  const name = invalid
    ? "Release Captain for the compozy, reloaddev and advos workspaces and everything after"
    : agent.label;
  const description = invalid ? `${agent.description}. `.repeat(8) : agent.description;
  return (
    <div className="flex flex-col gap-6 py-2">
      <div className="flex items-start gap-5">
        <Popover defaultOpen={popover !== undefined}>
          <PopoverTrigger
            aria-label="Change avatar"
            className="grid size-28 shrink-0 place-items-center rounded-xl bg-card shadow-card outline-none enabled:hover:bg-hover focus-visible:shadow-focus-ring"
            disabled={readOnly}
          >
            <AgentAvatar
              appearance={agent.appearance}
              label={agent.label}
              provider={agent.provider}
              quietIdle
              size={72}
              surfaceClassName="ring-card"
            />
          </PopoverTrigger>
          <PopoverContent align="start" className="w-96">
            <AvatarPopoverBody
              agent={agent}
              initial={popover ?? "generated"}
              initialColor={popoverColor}
            />
          </PopoverContent>
        </Popover>
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <Field data-invalid={invalid ? true : undefined}>
            <FieldLabel htmlFor="identity-name">Name</FieldLabel>
            <Input
              aria-invalid={invalid ? true : undefined}
              defaultValue={name}
              disabled={readOnly}
              id="identity-name"
            />
            {invalid ? <FieldError>Display names can have up to 80 characters.</FieldError> : null}
          </Field>
          <Field data-invalid={invalid ? true : undefined}>
            <FieldLabel htmlFor="identity-description">Description</FieldLabel>
            <Textarea
              aria-invalid={invalid ? true : undefined}
              defaultValue={description}
              disabled={readOnly}
              id="identity-description"
              rows={3}
            />
            {invalid ? <FieldError>Descriptions can have up to 280 characters.</FieldError> : null}
          </Field>
        </div>
      </div>
      <ImmutableIdentity
        hint={
          readOnly
            ? `${agent.owner} owns this definition. Update the extension to change it.`
            : "Duplicate this agent to choose another kind."
        }
        rows={[
          { label: "Slug", value: agent.name, mono: true },
          { label: "Kind", value: agent.kind === "persistent" ? "Persistent" : "Ephemeral" },
        ]}
      />
    </div>
  );
}
