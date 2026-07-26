import { Webhook } from "lucide-react";
import { useState } from "react";

import { SettingsGroup, type SettingsHookEntry } from "@/systems/settings";
import { Empty, Pill, PillGroup, SearchInput, Spinner, Switch } from "@agh/ui";

interface HooksSectionProps {
  hooks: SettingsHookEntry[];
  pendingHookName: string | null;
  hookError: string | null;
  canMutate: boolean;
  onToggle: (entry: SettingsHookEntry, nextEnabled: boolean) => void;
}

type HookStateFilter = "all" | "on" | "off";

function hookMatches(entry: SettingsHookEntry, query: string, state: HookStateFilter): boolean {
  const enabled = entry.declaration.enabled !== false;
  if (state === "on" && !enabled) return false;
  if (state === "off" && enabled) return false;
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return (
    entry.name.toLowerCase().includes(needle) ||
    entry.declaration.event.toLowerCase().includes(needle)
  );
}

export function HooksSection({
  hooks,
  pendingHookName,
  hookError,
  canMutate,
  onToggle,
}: HooksSectionProps) {
  const [query, setQuery] = useState("");
  const [stateFilter, setStateFilter] = useState<HookStateFilter>("all");
  const visible = hooks.filter(entry => hookMatches(entry, query, stateFilter));
  const isFiltered = query.trim() !== "" || stateFilter !== "all";

  return (
    <SettingsGroup
      data-testid="settings-page-hooks-section"
      title="Lifecycle hooks"
      description="Restart the daemon to re-read hook declarations. Enablement changes persist immediately."
    >
      {hookError ? (
        <span className="text-xs text-danger" data-testid="settings-page-hooks-error-message">
          {hookError}
        </span>
      ) : null}
      {hooks.length === 0 ? (
        <Empty
          icon={Webhook}
          title="No hooks registered"
          description="Add a hook declaration to ~/.agh/config.toml or a workspace overlay to register one."
          data-testid="settings-page-hooks-empty"
        />
      ) : (
        <div className="flex flex-col gap-3">
          <div
            className="flex flex-wrap items-center gap-2"
            data-testid="settings-page-hooks-listbar"
          >
            <span className="text-form-label text-subtle">
              Registered <span className="font-medium text-muted">{hooks.length}</span>
            </span>
            <SearchInput
              aria-label="Search hooks & events"
              className="h-7 w-56"
              data-testid="settings-page-hooks-search"
              onChange={setQuery}
              placeholder="Search hooks & events"
              value={query}
            />
            <span className="ml-auto">
              <PillGroup<HookStateFilter>
                aria-label="Filter hooks by state"
                items={[
                  { value: "all", label: "All", testId: "settings-page-hooks-filter-all" },
                  { value: "on", label: "On", testId: "settings-page-hooks-filter-on" },
                  { value: "off", label: "Off", testId: "settings-page-hooks-filter-off" },
                ]}
                onChange={setStateFilter}
                size="sm"
                value={stateFilter}
              />
            </span>
          </div>
          <ul
            className="overflow-hidden rounded-lg border border-line"
            data-testid="settings-page-hooks-list"
          >
            {visible.map(entry => (
              <HookRow
                key={entry.name}
                entry={entry}
                pending={pendingHookName === entry.name}
                canMutate={canMutate}
                onToggle={onToggle}
              />
            ))}
            {visible.length === 0 ? (
              <li
                className="px-4 py-3 text-form-label text-subtle"
                data-testid="settings-page-hooks-filter-empty"
              >
                {isFiltered ? "No hooks match the current filter." : "No hooks registered."}
              </li>
            ) : null}
          </ul>
        </div>
      )}
    </SettingsGroup>
  );
}

function HookRow({
  entry,
  pending,
  canMutate,
  onToggle,
}: {
  entry: SettingsHookEntry;
  pending: boolean;
  canMutate: boolean;
  onToggle: (entry: SettingsHookEntry, nextEnabled: boolean) => void;
}) {
  const declaration = entry.declaration;
  const enabled = declaration.enabled !== false;
  const matcherSummary = summarizeMatcher(declaration.matcher);
  const mode = declaration.mode === "sync" ? "blocking" : (declaration.mode ?? "async");
  const commandLine = declaration.command
    ? [declaration.command, ...(declaration.args ?? [])].join(" ")
    : null;

  return (
    <li
      className="grid grid-cols-[26px_minmax(0,1fr)_auto] items-center gap-3 border-b border-line-soft bg-canvas-soft px-4 py-2.5 last:border-b-0"
      data-testid={`settings-page-hooks-row-${entry.name}`}
    >
      <span
        aria-hidden="true"
        className="flex size-6.5 items-center justify-center rounded-sm bg-elevated text-subtle"
      >
        <Webhook className="size-3.5" />
      </span>
      <div className="flex min-w-0 flex-col gap-1">
        <span className="truncate font-mono text-sm text-fg">{entry.name}</span>
        <span className="flex min-w-0 flex-wrap items-center gap-1.5">
          <Pill mono size="xs" tone="info">
            {declaration.event}
          </Pill>
          <span className="font-mono text-mono-id text-muted">{mode}</span>
          {matcherSummary ? (
            <span
              className="truncate font-mono text-mono-id text-subtle"
              data-testid={`settings-page-hooks-row-${entry.name}-matcher`}
            >
              {matcherSummary}
            </span>
          ) : null}
          {commandLine ? (
            <span className="truncate font-mono text-mono-id text-faint">{commandLine}</span>
          ) : null}
        </span>
      </div>
      <div className="flex items-center justify-end gap-2">
        {pending ? <Spinner className="size-3 text-subtle" /> : null}
        <Switch
          data-testid={`settings-page-hooks-row-${entry.name}-toggle`}
          checked={enabled}
          disabled={pending || !canMutate}
          onCheckedChange={checked => onToggle(entry, checked)}
          aria-label={`Toggle hook ${entry.name}`}
        />
      </div>
    </li>
  );
}

function summarizeMatcher(matcher: SettingsHookEntry["declaration"]["matcher"]): string {
  const entries = Object.entries(matcher).filter(
    ([, value]) => value !== undefined && value !== null && value !== ""
  );
  if (entries.length === 0) return "";
  return entries.map(([key, value]) => `${key}=${String(value)}`).join(" · ");
}
