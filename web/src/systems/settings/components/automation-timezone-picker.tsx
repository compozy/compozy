import { useState } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectShell,
  CommandSelectTrigger,
} from "@compozy/ui";

interface AutomationTimezonePickerProps {
  value: string;
  hostTimezone?: string;
  onChange: (zone: string) => void;
}

export function AutomationTimezonePicker({
  value,
  hostTimezone,
  onChange,
}: AutomationTimezonePickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [supportedZones] = useState(() => Intl.supportedValuesOf("timeZone"));
  const searchedZone = resolveSearchTimezone(search);
  const zones = [
    ...new Set(
      ["UTC", ...supportedZones, value, hostTimezone, searchedZone].filter((zone): zone is string =>
        Boolean(zone)
      )
    ),
  ].sort();

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);
    setSearch("");
  }

  return (
    <div className="flex min-w-0 flex-col items-start gap-2">
      <CommandSelect open={open} onOpenChange={handleOpenChange}>
        <CommandSelectTrigger
          aria-label="Schedule time zone"
          className="w-56 max-w-full font-mono"
          data-testid="settings-page-automation-timezone-picker"
          label={value}
          selected
        />
        <CommandSelectShell
          inputPlaceholder="Search time zones…"
          inputProps={{ value: search, onValueChange: setSearch }}
          commandProps={{ label: "Search time zones" }}
        >
          <CommandList>
            <CommandEmpty>No time zones found.</CommandEmpty>
            {zones.map(zone => (
              <CommandItem
                key={zone}
                value={zone}
                keywords={zone === searchedZone ? [search] : undefined}
                onSelect={() => {
                  onChange(zone);
                  handleOpenChange(false);
                }}
              >
                {zone}
              </CommandItem>
            ))}
          </CommandList>
        </CommandSelectShell>
      </CommandSelect>
      {hostTimezone ? (
        <Button
          variant="link"
          size="sm"
          className="h-auto whitespace-normal p-0 text-left"
          disabled={value === hostTimezone}
          onClick={() => onChange(hostTimezone)}
        >
          Use this machine’s time zone ({hostTimezone})
        </Button>
      ) : null}
    </div>
  );
}

function resolveSearchTimezone(search: string): string | undefined {
  if (!search.trim()) return undefined;
  try {
    const zone = new Intl.DateTimeFormat("en", { timeZone: search.trim() }).resolvedOptions()
      .timeZone;
    // Intl also accepts numeric offsets, which are not IANA names accepted by the daemon.
    return /^[+-]/.test(zone) ? undefined : zone;
  } catch {
    return undefined;
  }
}
