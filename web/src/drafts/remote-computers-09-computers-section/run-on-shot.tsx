import {
  Button,
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSelectGroup,
  CommandSeparator,
  Field,
  FieldLabel,
  Pill,
  StatusDot,
  cn,
  toast,
} from "@compozy/ui";
import { Laptop, Scale } from "@compozy/ui/icons";
import { ClaudeLogo } from "@compozy/ui/logos";
import { useMemo, useState } from "react";

import { ComputerLoadMeter } from "@/systems/computers/components/computer-load-meter";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { RunOnPicker, type RunOnTarget } from "@/systems/computers/components/run-on-picker";
import { RunOnComputerItem } from "@/systems/computers/components/run-on-picker-item";
import { COMPUTERS_PRO, LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  projectsForTarget,
  resolveAutoComputer,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

const PICKER_COMPUTERS = COMPUTERS_PRO.slice(0, 4);

/**
 * Real Run on showcase inside the landing's `ProductWindow`.
 * Uses the real `RunOnPicker` trigger, and when `pickerDefaultOpen` is true for the capture state,
 * anchors the open menu inline directly beneath its trigger inside the window (using the real
 * `RunOnComputerItem` and `ComputerLoadMeter` components) so it never floats detached over page copy.
 */
export function RunOnShot({
  pickerDefaultOpen: initialPickerOpen,
}: {
  pickerDefaultOpen: boolean;
}) {
  const [target, setTarget] = useState<RunOnTarget>("auto");
  const [query, setQuery] = useState("");
  const [inlineMenuOpen, setInlineMenuOpen] = useState(initialPickerOpen);

  const projects = useMemo(() => projectsForTarget(target), [target]);
  const activeProject = projects[0]?.name ?? "api";
  const autoResolution = useMemo(
    () => resolveAutoComputer(activeProject, LOCAL_COMPUTER, COMPUTERS_PRO),
    [activeProject]
  );

  const handleSelect = (next: RunOnTarget) => {
    setTarget(next);
    setQuery("");
  };

  return (
    <div className="flex h-full flex-col bg-canvas text-fg">
      <div className="flex h-11 shrink-0 items-center justify-between border-b border-line bg-rail px-4">
        <div className="flex items-center gap-3">
          <div aria-hidden className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-full bg-line-strong" />
            <span className="size-2.5 rounded-full bg-line-strong" />
            <span className="size-2.5 rounded-full bg-line-strong" />
          </div>
          <span className="text-small-body font-medium text-fg">New session</span>
        </div>
        <span className="font-mono text-mono-id text-subtle">4 computers online</span>
      </div>

      <div className="grid flex-1 gap-5 overflow-y-auto p-5 md:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)] md:p-6">
        <div className="flex flex-col gap-4 rounded-lg bg-card p-5 shadow-card">
          <Field>
            <FieldLabel htmlFor="landing-run-on">Run on</FieldLabel>
            {inlineMenuOpen ? (
              <div className="flex flex-col gap-1.5">
                <div
                  onClick={event => {
                    event.preventDefault();
                    setInlineMenuOpen(false);
                  }}
                >
                  <RunOnPicker
                    allowAuto
                    autoResolution={autoResolution}
                    computers={PICKER_COMPUTERS}
                    defaultOpen={false}
                    loadBalancingEnabled
                    localComputer={LOCAL_COMPUTER}
                    onChange={handleSelect}
                    triggerId="landing-run-on"
                    value={target}
                  />
                </div>
                <div className="overflow-hidden rounded-md border border-line-strong bg-popover shadow-overlay">
                  <Command className="bg-popover">
                    <CommandInput
                      onValueChange={setQuery}
                      placeholder="Search computers…"
                      value={query}
                    />
                    <CommandList className="max-h-[268px] overflow-y-auto bg-popover">
                      <CommandEmpty>No matching computer.</CommandEmpty>
                      <CommandSelectGroup heading="Default">
                        <CommandItem
                          className="items-center gap-2.5 py-1.5"
                          data-checked={target === "auto" ? "true" : "false"}
                          onSelect={() => handleSelect("auto")}
                          value={`Auto load balancing ${autoResolution?.computerName ?? ""}`}
                        >
                          <Scale aria-hidden className="size-4 shrink-0 text-subtle" />
                          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                            <span className="flex items-center gap-1.5 text-small-body font-medium text-fg">
                              <span>Auto</span>
                              {autoResolution ? (
                                <span className="font-normal text-muted-foreground">
                                  → {autoResolution.computerName}
                                </span>
                              ) : null}
                            </span>
                            <span className="truncate text-eyebrow text-subtle">
                              Picks the least busy computer
                            </span>
                          </span>
                        </CommandItem>

                        <CommandItem
                          className="items-center gap-2.5 py-1.5"
                          data-checked={target === "local" ? "true" : "false"}
                          onSelect={() => handleSelect("local")}
                          value={`This computer ${LOCAL_COMPUTER.name} local`}
                        >
                          <ComputerPlatformIcon
                            className="size-4 shrink-0"
                            iconKind={LOCAL_COMPUTER.iconKind}
                            platform={LOCAL_COMPUTER.platform}
                            size="sm"
                            tone="muted"
                          />
                          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                            <span className="truncate text-small-body font-medium text-fg">
                              This computer
                            </span>
                            <span className="flex items-center gap-1.5 truncate text-eyebrow text-subtle">
                              <StatusDot size="sm" tone="success" />
                              <span className="truncate">{LOCAL_COMPUTER.name}</span>
                            </span>
                          </span>
                          <ComputerLoadMeter
                            computerName={LOCAL_COMPUTER.name}
                            load={LOCAL_COMPUTER.load}
                          />
                        </CommandItem>
                      </CommandSelectGroup>

                      <CommandSeparator />

                      <CommandSelectGroup heading="Computers">
                        {PICKER_COMPUTERS.map(computer => (
                          <RunOnComputerItem
                            computer={computer}
                            highlightReconnect={false}
                            key={computer.id}
                            onSelect={handleSelect}
                            reconnecting={false}
                            selected={target === computer.id}
                            testIdPrefix="landing-run-on"
                          />
                        ))}
                      </CommandSelectGroup>
                    </CommandList>
                  </Command>
                </div>
              </div>
            ) : (
              <RunOnPicker
                allowAuto
                autoResolution={autoResolution}
                computers={PICKER_COMPUTERS}
                defaultOpen={false}
                loadBalancingEnabled
                localComputer={LOCAL_COMPUTER}
                onChange={handleSelect}
                triggerId="landing-run-on"
                value={target}
              />
            )}
          </Field>

          {!inlineMenuOpen ? (
            <div className="grid gap-3.5 sm:grid-cols-2">
              <div className="rounded-md border border-line-soft bg-sunken px-3.5 py-2.5">
                <p className="m-0 text-eyebrow text-subtle">Project</p>
                <p className="m-0 mt-1 text-small-body font-medium text-fg">{activeProject}</p>
              </div>
              <div className="rounded-md border border-line-soft bg-sunken px-3.5 py-2.5">
                <p className="m-0 text-eyebrow text-subtle">Agent</p>
                <p className="m-0 mt-1 inline-flex items-center gap-1.5 text-small-body font-medium text-fg">
                  <ClaudeLogo className="size-3.5" aria-hidden />
                  Claude Code
                </p>
              </div>
            </div>
          ) : null}

          <div className="mt-auto flex items-center justify-between border-t border-line-soft pt-3.5">
            <span className="text-eyebrow text-muted-foreground">
              {target === "auto" && autoResolution
                ? `Auto resolves to ${autoResolution.computerName}`
                : target === "local"
                  ? "Runs on MacBook Pro"
                  : `Runs on ${COMPUTERS_PRO.find(c => c.id === target)?.name ?? "devbox"}`}
            </span>
            <Button
              size="sm"
              type="button"
              onClick={() => toast.message("Starts session on selected computer")}
            >
              Start session
            </Button>
          </div>
        </div>

        <div className="flex flex-col justify-between gap-4 rounded-lg border border-line-soft bg-sunken p-5">
          <div>
            <p className="m-0 text-eyebrow font-medium text-fg">Live fleet capacity</p>
            <p className="m-0 mt-1 text-meta text-muted-foreground">
              Auto scores every connected computer that holds the project and picks the one with the
              most free CPU and memory.
            </p>

            <div className="mt-4 grid grid-cols-2 gap-3 border-y border-line-soft py-3">
              <div>
                <p className="m-0 text-badge text-subtle">Project</p>
                <p className="m-0 mt-0.5 text-small-body font-medium text-fg">{activeProject}</p>
              </div>
              <div>
                <p className="m-0 text-badge text-subtle">Agent</p>
                <p className="m-0 mt-0.5 inline-flex items-center gap-1.5 text-small-body font-medium text-fg">
                  <ClaudeLogo className="size-3.5" aria-hidden />
                  Claude Code
                </p>
              </div>
            </div>

            <ul className="m-0 mt-4 list-none divide-y divide-line-soft p-0">
              {autoResolution?.candidates.map(candidate => (
                <li
                  key={candidate.id}
                  className="flex items-center justify-between gap-3 py-2.5 first:pt-0 last:pb-0"
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <Laptop aria-hidden className="size-3.5 shrink-0 text-subtle" />
                    <span
                      className={cn(
                        "truncate text-small-body",
                        candidate.winner ? "font-medium text-fg" : "text-muted-foreground"
                      )}
                    >
                      {candidate.name}
                    </span>
                    {candidate.winner ? (
                      <Pill form="tint" size="xs" tone="success">
                        Chosen
                      </Pill>
                    ) : null}
                  </div>
                  <ComputerLoadMeter computerName={candidate.name} load={candidate.load} />
                </li>
              ))}
            </ul>
          </div>
          <p className="m-0 border-t border-line-soft pt-3 text-eyebrow text-subtle">
            Works in Session create, Kanban New session, and scheduled automations.
          </p>
        </div>
      </div>
    </div>
  );
}
