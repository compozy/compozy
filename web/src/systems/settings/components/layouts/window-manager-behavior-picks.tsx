import type { ComponentType, Dispatch, SetStateAction } from "react";

import { Switch, cn } from "@agh/ui";

import type { WindowManagerConfig, WindowManagerDragModifier } from "@/systems/os";

import { SettingRow } from "../setting-row";
import {
  DiagramClickAndKeys,
  DiagramCrossfade,
  DiagramDragGroup,
  DiagramDragWindow,
  DiagramFloatOnTop,
  DiagramFoldIntoStack,
  DiagramInstant,
  DiagramKeysOnly,
  DiagramRefuse,
  DiagramSlide,
  DiagramSplitBesideFocus,
} from "./window-manager-behavior-diagrams";

type ConfigDraft = Dispatch<SetStateAction<WindowManagerConfig>>;

interface PickOption<TValue extends string> {
  value: TValue;
  label: string;
  diagram: ComponentType;
}

interface PickRow<TKey extends keyof WindowManagerConfig> {
  key: TKey;
  label: string;
  description: string;
  options: ReadonlyArray<PickOption<Extract<WindowManagerConfig[TKey], string>>>;
}

const NEW_WINDOW: PickRow<"newWindowPolicy"> = {
  key: "newWindowPolicy",
  label: "New windows",
  description: "Where a window lands when it opens without a home.",
  options: [
    { value: "floating", label: "Float on top", diagram: DiagramFloatOnTop },
    { value: "beside_focus", label: "Split beside focus", diagram: DiagramSplitBesideFocus },
  ],
};

const SMALL_VIEWPORT: PickRow<"smallViewportPolicy"> = {
  key: "smallViewportPolicy",
  label: "Not enough room",
  description: "When the screen can no longer hold every tile side by side.",
  options: [
    { value: "stack", label: "Fold into a stack", diagram: DiagramFoldIntoStack },
    { value: "reject", label: "Refuse the layout", diagram: DiagramRefuse },
  ],
};

const FOCUS: PickRow<"focusPolicy"> = {
  key: "focusPolicy",
  label: "Taking focus",
  description: "Whether a click can move focus, or only the keyboard.",
  options: [
    { value: "click_directional", label: "Click and keys", diagram: DiagramClickAndKeys },
    { value: "directional", label: "Keys only", diagram: DiagramKeysOnly },
  ],
};

const DRAG_AWAY: PickRow<"dragAwayPolicy"> = {
  key: "dragAwayPolicy",
  label: "Dragging a tiled window",
  description: "What comes along when you pull one out of its group.",
  options: [
    { value: "window", label: "Just that window", diagram: DiagramDragWindow },
    { value: "group", label: "Its whole group", diagram: DiagramDragGroup },
  ],
};

const TRANSITION: PickRow<"desktopTransition"> = {
  key: "desktopTransition",
  label: "Switching desktops",
  description: "How this browser moves between desktops.",
  options: [
    { value: "slide", label: "Slide", diagram: DiagramSlide },
    { value: "crossfade", label: "Crossfade", diagram: DiagramCrossfade },
    { value: "instant", label: "Instant", diagram: DiagramInstant },
  ],
};

const MODIFIER_KEYS: ReadonlyArray<{
  value: WindowManagerDragModifier;
  symbol: string;
  name: string;
}> = [
  { value: "alt", symbol: "⌥", name: "Option" },
  { value: "control", symbol: "⌃", name: "Control" },
  { value: "meta", symbol: "⌘", name: "Command" },
  { value: "shift", symbol: "⇧", name: "Shift" },
  { value: "none", symbol: "None", name: "None" },
];

const TOGGLES = [
  {
    key: "focusWrap",
    label: "Wrap directional focus",
    description: "At the last window in a direction, continue from the opposite edge.",
  },
  {
    key: "focusFollowsPointer",
    label: "Focus follows the pointer",
    description: "A window takes focus as soon as the pointer enters it.",
  },
  {
    key: "raiseOnFocus",
    label: "Raise on focus",
    description: "Bring a focused floating window above its peers.",
  },
] as const;

/**
 * Seven enums that used to be seven selects. Each option is the arrangement it
 * produces, so the choice is read rather than decoded — and the two drag
 * modifiers are the keys themselves.
 */
export function WindowManagerBehaviorPicks({
  draft,
  setDraft,
}: {
  draft: WindowManagerConfig;
  setDraft: ConfigDraft;
}) {
  return (
    <>
      <DiagramPickRow draft={draft} row={NEW_WINDOW} setDraft={setDraft} />
      <DiagramPickRow draft={draft} row={SMALL_VIEWPORT} setDraft={setDraft} />
      <DiagramPickRow draft={draft} row={FOCUS} setDraft={setDraft} />
      <DiagramPickRow draft={draft} row={DRAG_AWAY} setDraft={setDraft} />
      <DiagramPickRow draft={draft} row={TRANSITION} setDraft={setDraft} />

      <ModifierRow
        description="Hold this while dragging to take the group instead of one window."
        label="Move the whole group"
        value={draft.groupMoveModifier}
        onChange={groupMoveModifier => setDraft(current => ({ ...current, groupMoveModifier }))}
      />
      <ModifierRow
        description="Hold this while dropping onto another window to trade places."
        label="Swap two windows"
        value={draft.swapModifier}
        onChange={swapModifier => setDraft(current => ({ ...current, swapModifier }))}
      />

      {TOGGLES.map(toggle => (
        <SettingRow
          control={
            <Switch
              aria-label={toggle.label}
              checked={draft[toggle.key]}
              onCheckedChange={checked =>
                setDraft(current => ({ ...current, [toggle.key]: checked }))
              }
            />
          }
          data-testid={`window-manager-${toggle.key}`}
          description={toggle.description}
          key={toggle.key}
          label={toggle.label}
        />
      ))}
    </>
  );
}

function DiagramPickRow<TKey extends keyof WindowManagerConfig>({
  row,
  draft,
  setDraft,
}: {
  row: PickRow<TKey>;
  draft: WindowManagerConfig;
  setDraft: ConfigDraft;
}) {
  const current = draft[row.key];

  return (
    <div className="flex flex-col gap-3 border-t border-line-soft px-4 py-3.5 first:border-t-0 min-[720px]:flex-row min-[720px]:items-center min-[720px]:justify-between min-[720px]:gap-6">
      <div className="min-w-0 flex-1">
        <p className="text-ws-name font-medium text-fg-strong">{row.label}</p>
        <p className="mt-0.5 max-w-setting-description text-form-label leading-normal text-muted">
          {row.description}
        </p>
      </div>
      <div
        aria-label={row.label}
        className="flex shrink-0 flex-wrap gap-1.5"
        data-testid={`window-manager-pick-${String(row.key)}`}
        role="radiogroup"
      >
        {row.options.map(option => {
          const Diagram = option.diagram;
          const selected = current === option.value;
          return (
            <button
              aria-checked={selected}
              className={cn(
                "flex w-26 flex-col items-start gap-1.5 rounded-md border border-line px-2.5 pt-2 pb-2.5",
                "bg-btn-default-fill text-left text-form-label font-medium text-muted",
                "transition-colors duration-base ease-out",
                "hover:border-line-strong hover:bg-btn-default-hover hover:text-fg",
                "focus-visible:outline-none focus-visible:shadow-focus-ring",
                selected && "border-accent-dim bg-accent-tint text-accent-strong"
              )}
              data-testid={`window-manager-pick-${String(row.key)}-${option.value}`}
              key={option.value}
              role="radio"
              type="button"
              onClick={() =>
                setDraft(
                  current => ({ ...current, [row.key]: option.value }) as WindowManagerConfig
                )
              }
            >
              <Diagram />
              <span className={cn("leading-tight", selected && "text-fg-strong")}>
                {option.label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function ModifierRow({
  label,
  description,
  value,
  onChange,
}: {
  label: string;
  description: string;
  value: WindowManagerDragModifier;
  onChange: (next: WindowManagerDragModifier) => void;
}) {
  return (
    <SettingRow
      control={
        <div
          aria-label={label}
          className="inline-flex gap-0.5 rounded-md border border-line-soft bg-canvas-tint p-0.5"
          role="radiogroup"
        >
          {MODIFIER_KEYS.map(key => (
            <button
              aria-checked={key.value === value}
              aria-label={key.name}
              className={cn(
                "inline-flex h-6.5 min-w-8.5 items-center justify-center rounded-sm px-2",
                "font-mono text-ws-name text-muted transition-colors duration-base ease-out",
                "hover:bg-row-hover hover:text-fg",
                "focus-visible:outline-none focus-visible:shadow-focus-ring",
                key.value === "none" && "font-sans text-form-label font-medium",
                key.value === value && "bg-elevated text-fg-strong shadow-highlight"
              )}
              key={key.value}
              role="radio"
              type="button"
              onClick={() => onChange(key.value)}
            >
              {key.symbol}
            </button>
          ))}
        </div>
      }
      description={description}
      label={label}
    />
  );
}
