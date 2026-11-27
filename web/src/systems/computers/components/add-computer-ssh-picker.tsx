import { useState } from "react";
import { AlertCircle, ServerIcon, Terminal } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Disclosure,
  Field,
  FieldDescription,
  FieldLabel,
  FormSection,
  Input,
  Pill,
} from "@compozy/ui";

import type { SshHostSuggestionFixture } from "../mocks/remote-computers-fixtures";

export interface SshTargetDraft {
  host: string;
  user: string;
  port: string;
  name: string;
}

export interface AddComputerSshPickerProps {
  suggestions: readonly SshHostSuggestionFixture[];
  value: SshTargetDraft;
  onChange: (next: SshTargetDraft) => void;
  /** Default combobox open state for prototype review. */
  defaultPickerOpen?: boolean;
  /** Optional inline link to jump to "Link with a command" when platform is unsupported. */
  onSwitchToCommand?: () => void;
}

function parseSshTarget(raw: string, value: SshTargetDraft): SshTargetDraft | null {
  const clean = raw.trim();
  if (!clean) return null;
  const atIndex = clean.indexOf("@");
  const parsedUser = atIndex > 0 ? clean.slice(0, atIndex) : value.user;
  const hostPart = atIndex > 0 ? clean.slice(atIndex + 1) : clean;
  const colonIndex = hostPart.lastIndexOf(":");
  const parsedHost = colonIndex > 0 ? hostPart.slice(0, colonIndex) : hostPart;
  const parsedPort = colonIndex > 0 ? hostPart.slice(colonIndex + 1) : value.port || "22";

  return {
    host: parsedHost,
    user: parsedUser,
    port: parsedPort,
    name: parsedHost.split(".")[0] || parsedHost,
  };
}

function sshHostLabel(value: SshTargetDraft, matched?: SshHostSuggestionFixture) {
  return value.host
    ? matched
      ? `${matched.alias} (${matched.user}@${matched.hostname})`
      : value.user
        ? `${value.user}@${value.host}${value.port && value.port !== "22" ? `:${value.port}` : ""}`
        : value.host
    : undefined;
}

/**
 * SSH host selector fed by `~/.ssh/config` and `~/.ssh/known_hosts`, with
 * free-entry support and compact inline disclosure for User, Port, and Name.
 */
export function AddComputerSshPicker({
  suggestions,
  value,
  onChange,
  defaultPickerOpen = false,
  onSwitchToCommand,
}: AddComputerSshPickerProps) {
  const [open, setOpen] = useState(defaultPickerOpen);
  const [search, setSearch] = useState("");

  const configHosts = suggestions.filter(item => item.source === "ssh_config");
  const knownHosts = suggestions.filter(item => item.source === "known_hosts");
  const matched = suggestions.find(
    item => item.alias === value.host || item.hostname === value.host
  );

  const trimmedSearch = search.trim();
  const searchMatchesExisting = suggestions.some(
    item =>
      item.alias.toLowerCase() === trimmedSearch.toLowerCase() ||
      item.hostname.toLowerCase() === trimmedSearch.toLowerCase()
  );

  const selectSuggestion = (suggestion: SshHostSuggestionFixture) => {
    onChange({
      host: suggestion.alias,
      user: suggestion.user,
      port: String(suggestion.port),
      name: suggestion.alias,
    });
    setOpen(false);
    setSearch("");
  };

  const selectCustomHost = (raw: string) => {
    const target = parseSshTarget(raw, value);
    if (!target) return;
    onChange(target);
    setOpen(false);
    setSearch("");
  };

  const triggerLabel = sshHostLabel(value, matched);

  return (
    <FormSection
      description="Suggestions come from your local ~/.ssh/config and recent known hosts, or type any hostname."
      icon={Terminal}
      title="SSH host"
    >
      <Field>
        <FieldLabel htmlFor="add-computer-ssh-host">Host</FieldLabel>
        <CommandSelect onOpenChange={setOpen} open={open}>
          <CommandSelectTrigger
            icon={<ServerIcon aria-hidden="true" className="size-3.5" />}
            id="add-computer-ssh-host"
            label={triggerLabel}
            placeholder="Pick an SSH alias or enter user@host"
            selected={Boolean(value.host)}
          />
          <CommandSelectShell
            inputPlaceholder="Search ~/.ssh/config or type user@host…"
            inputProps={{
              onValueChange: setSearch,
              value: search,
            }}
          >
            <CommandList>
              <CommandEmpty>No matching SSH hosts.</CommandEmpty>
              {configHosts.length > 0 ? (
                <CommandSelectGroup heading="From ~/.ssh/config">
                  {configHosts.map(item => (
                    <CommandItem
                      data-checked={value.host === item.alias ? "true" : undefined}
                      key={item.alias}
                      onSelect={() => selectSuggestion(item)}
                      value={`${item.alias} ${item.user}@${item.hostname}`}
                    >
                      <ServerIcon aria-hidden="true" className="size-3.5 text-subtle" />
                      <div className="flex min-w-0 flex-1 items-baseline gap-2">
                        <span className="truncate font-medium text-fg">{item.alias}</span>
                        <span className="truncate font-mono text-mono-id text-muted-foreground">
                          {item.user}@{item.hostname}
                          {item.port !== 22 ? `:${item.port}` : ""}
                        </span>
                      </div>
                      {item.alreadyAdded ? (
                        <Pill size="xs" tone="neutral">
                          Already added
                        </Pill>
                      ) : null}
                    </CommandItem>
                  ))}
                </CommandSelectGroup>
              ) : null}

              {knownHosts.length > 0 ? (
                <CommandSelectGroup heading="Recent & known hosts">
                  {knownHosts.map(item => (
                    <CommandItem
                      data-checked={value.host === item.alias ? "true" : undefined}
                      key={item.alias}
                      onSelect={() => selectSuggestion(item)}
                      value={`${item.alias} ${item.user}@${item.hostname}`}
                    >
                      <ServerIcon aria-hidden="true" className="size-3.5 text-subtle" />
                      <div className="flex min-w-0 flex-1 items-baseline gap-2">
                        <span className="truncate font-medium text-fg">{item.alias}</span>
                        <span className="truncate font-mono text-mono-id text-muted-foreground">
                          {item.user}@{item.hostname}
                        </span>
                      </div>
                    </CommandItem>
                  ))}
                </CommandSelectGroup>
              ) : null}

              {trimmedSearch && !searchMatchesExisting ? (
                <CommandSelectGroup heading="Manual host">
                  <CommandItem
                    onSelect={() => selectCustomHost(trimmedSearch)}
                    value={trimmedSearch}
                  >
                    <Terminal aria-hidden="true" className="size-3.5 text-subtle" />
                    <span className="truncate">
                      Connect to{" "}
                      <span className="font-mono text-mono-id font-medium text-fg">
                        {trimmedSearch}
                      </span>
                    </span>
                  </CommandItem>
                </CommandSelectGroup>
              ) : null}
            </CommandList>
          </CommandSelectShell>
        </CommandSelect>
        <FieldDescription>
          CompozyOS installs the verified release over SSH and keeps agents running after you quit.
        </FieldDescription>
      </Field>

      {matched?.alreadyAdded ? (
        <Alert variant="warning">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>{matched.alias} is already in your computers</AlertTitle>
          <AlertDescription>
            Choose another host, or open {matched.alias} from your Computers list.
          </AlertDescription>
        </Alert>
      ) : null}

      {matched?.unsupportedPlatform ? (
        <Alert variant="warning">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>Not supported over SSH</AlertTitle>
          <AlertDescription className="flex flex-col gap-1.5">
            <span>
              This computer's system isn't supported over SSH. Link it with{" "}
              <span className="font-mono text-mono-id text-fg">compozy remote link</span> instead.
            </span>
            {onSwitchToCommand ? (
              <button
                className="w-fit cursor-pointer font-medium text-fg underline underline-offset-3 focus-visible:shadow-focus-ring focus-visible:outline-none"
                onClick={onSwitchToCommand}
                type="button"
              >
                Switch to Link with a command
              </button>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      <Disclosure label="User, port, and display name" variant="inline">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Field>
            <FieldLabel htmlFor="add-computer-ssh-user">User</FieldLabel>
            <Input
              id="add-computer-ssh-user"
              onChange={event => onChange({ ...value, user: event.target.value })}
              placeholder="Default from SSH config"
              value={value.user}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="add-computer-ssh-port">Port</FieldLabel>
            <Input
              id="add-computer-ssh-port"
              inputMode="numeric"
              onChange={event => onChange({ ...value, port: event.target.value })}
              placeholder="22"
              value={value.port}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="add-computer-ssh-name">Display name</FieldLabel>
            <Input
              id="add-computer-ssh-name"
              onChange={event => onChange({ ...value, name: event.target.value })}
              placeholder={value.host || "devbox"}
              value={value.name}
            />
          </Field>
        </div>
      </Disclosure>
    </FormSection>
  );
}
