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

function GeneratedPane({ agent }: { agent: DraftAgent }) {
  const [shuffles, setShuffles] = useState(0);
  const seed = shuffles === 0 ? agent.name : `${agent.name}-${shuffles}`;
  return (
    <div className="flex items-center gap-3 rounded-md bg-sunken p-3">
      <AgentAvatar
        appearance={{ avatar: "generated", seed }}
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

function AvatarPopoverBody({ agent, initial }: { agent: DraftAgent; initial: Mode }) {
  const [mode, setMode] = useState<Mode>(initial);
  return (
    <div className="flex flex-col gap-3">
      <PillGroup items={MODE_ITEMS} onChange={setMode} size="md" value={mode} />
      {mode === "generated" ? <GeneratedPane agent={agent} /> : <SymbolPane />}
    </div>
  );
}

export function IdentityEditor({
  agent,
  popover,
  invalid = false,
  readOnly = false,
}: {
  agent: DraftAgent;
  popover?: Mode;
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
            <AvatarPopoverBody agent={agent} initial={popover ?? "generated"} />
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
