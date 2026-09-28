import { Webhook } from "lucide-react";
import { useState } from "react";

import { SettingsGroup, type SettingsHookEntry } from "@/systems/settings";
import {
  buttonVariants,
  Empty,
  ListingRow,
  Pill,
  PillGroup,
  SearchInput,
  Spinner,
  Switch,
} from "@compozy/ui";

const HOOKS_DOCS_URL = "https://compozy.com/docs/hooks";

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
      description="Restart CompozyOS to re-read hook declarations. Enablement changes persist immediately."
    >
      {hookError ? (
        <span
          className="text-form-hint text-danger"
          data-testid="settings-page-hooks-error-message"
        >
          {hookError}
        </span>
      ) : null}
      {hooks.length === 0 ? (
        <Empty
          icon={Webhook}
          title="No hooks yet"
          description="Hooks are added in your settings file."
          action={
            <a
              className={buttonVariants({ size: "sm", variant: "outline" })}
              href={HOOKS_DOCS_URL}
              rel="noreferrer"
              target="_blank"
            >
              Read the docs
            </a>
          }
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
              containerClassName="w-56"
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
              <li className="border-b border-line-soft last:border-b-0" key={entry.name}>
                <HookRow
                  entry={entry}
                  pending={pendingHookName === entry.name}
                  canMutate={canMutate}
                  onToggle={onToggle}
                />
              </li>
            ))}
            {visible.length === 0 ? (
              <li
                className="px-4 py-3 text-form-label text-subtle"
                data-testid="settings-page-hooks-filter-empty"
              >
                {isFiltered ? "No hooks match the current filter." : "No hooks yet."}
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
    <ListingRow
      className="border-b-0 bg-canvas-soft py-2.5"
      data-testid={`settings-page-hooks-row-${entry.name}`}
      interactive={false}
    >
      <ListingRow.Icon>
        <Webhook className="size-3.5" />
      </ListingRow.Icon>
      <ListingRow.Main className="flex flex-col gap-1">
        <ListingRow.Title mono>{entry.name}</ListingRow.Title>
        <span className="flex min-w-0 flex-wrap items-center gap-1.5">
          <Pill mono size="xs" tone="neutral">
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
            <span className="truncate font-mono text-mono-id text-subtle">{commandLine}</span>
          ) : null}
        </span>
      </ListingRow.Main>
      <ListingRow.Trail className="justify-end gap-2">
        {pending ? <Spinner className="size-3 text-subtle" /> : null}
        <Switch
          data-testid={`settings-page-hooks-row-${entry.name}-toggle`}
          checked={enabled}
          disabled={pending || !canMutate}
          onCheckedChange={checked => onToggle(entry, checked)}
          aria-label={`Toggle hook ${entry.name}`}
        />
      </ListingRow.Trail>
    </ListingRow>
  );
}

function summarizeMatcher(matcher: SettingsHookEntry["declaration"]["matcher"]): string {
  const entries = Object.entries(matcher).filter(
    ([, value]) => value !== undefined && value !== null && value !== ""
  );
  if (entries.length === 0) return "";
  return entries.map(([key, value]) => `${key}=${String(value)}`).join(" · ");
}
