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
  const [supportedZones] = useState(() => Intl.supportedValuesOf("timeZone"));
  const zones = [
    ...new Set(
      ["UTC", ...supportedZones, value, hostTimezone].filter((zone): zone is string =>
        Boolean(zone)
      )
    ),
  ].sort();

  return (
    <div className="flex min-w-0 flex-col items-start gap-2">
      <CommandSelect open={open} onOpenChange={setOpen}>
        <CommandSelectTrigger
          aria-label="Schedule time zone"
          className="w-56 max-w-full font-mono"
          data-testid="settings-page-automation-timezone-picker"
          label={value}
          selected
        />
        <CommandSelectShell
          inputPlaceholder="Search time zones…"
          commandProps={{ label: "Search time zones" }}
        >
          <CommandList>
            <CommandEmpty>No time zones found.</CommandEmpty>
            {zones.map(zone => (
              <CommandItem
                key={zone}
                value={zone}
                onSelect={() => {
                  onChange(zone);
                  setOpen(false);
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
