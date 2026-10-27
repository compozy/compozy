import { useState } from "react";

import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
} from "@compozy/ui";

import {
  FONT_CATALOG,
  cssFontFamily,
  fontLabel,
  isStorableFontFamily,
  queryInstalledFonts,
  supportsInstalledFonts,
  type FontRole,
  type InstalledFonts,
} from "@/systems/theme";

interface FontPickerProps {
  role: FontRole;
  /** Stored family; `""` is the built-in default. */
  value: string;
  label: string;
  onChange: (family: string) => void;
  "data-testid"?: string;
}

/**
 * One Appearance font: the built-in default, the curated catalog, and — where
 * the Local Font Access API exists (the desktop app, Chromium) — the fonts
 * installed on this device, read on the first open. Every row previews in its
 * own face; the Code picker only lists monospace families.
 */
export function FontPicker({
  role,
  value,
  label,
  onChange,
  "data-testid": testId,
}: FontPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [installed, setInstalled] = useState<InstalledFonts | null>(null);

  const catalog = FONT_CATALOG.filter(entry => entry.roles.includes(role));
  const catalogFamilies = new Set(catalog.map(entry => entry.family));
  const installedFamilies = (installed?.families ?? []).filter(
    family =>
      !catalogFamilies.has(family) &&
      isStorableFontFamily(family) &&
      (role === "sans" || installed?.monospace.has(family) === true)
  );

  // A family chosen on another device (or set through the CLI) stays visible
  // even when this device does not list it.
  const orphan = value !== "" && !catalogFamilies.has(value) && !installedFamilies.includes(value);

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen);
    setSearch("");
    // The permission prompt (browsers) needs this click's user activation.
    if (nextOpen && installed === null && supportsInstalledFonts()) {
      void queryInstalledFonts().then(setInstalled);
    }
  }

  function choose(family: string) {
    onChange(family);
    handleOpenChange(false);
  }

  const renderItem = (family: string, itemLabel: string) => (
    <CommandItem
      key={family || "default"}
      value={family || "default"}
      keywords={[itemLabel]}
      data-checked={family === value}
      style={{ fontFamily: family ? cssFontFamily(family) : undefined }}
      onSelect={() => choose(family)}
    >
      <span className="truncate">{itemLabel}</span>
    </CommandItem>
  );

  return (
    <div className="flex min-w-0 flex-col items-start gap-2">
      <CommandSelect open={open} onOpenChange={handleOpenChange}>
        <CommandSelectTrigger
          aria-label={label}
          className="w-56 max-w-full"
          data-testid={testId}
          selected
        >
          <span
            className="min-w-0 flex-1 truncate text-left text-small-body text-fg"
            style={{ fontFamily: value ? cssFontFamily(value) : undefined }}
          >
            {fontLabel(value, role)}
          </span>
        </CommandSelectTrigger>
        <CommandSelectShell
          inputPlaceholder="Search fonts…"
          inputProps={{ value: search, onValueChange: setSearch }}
          commandProps={{ label: `Search ${label.toLowerCase()} fonts` }}
        >
          <CommandList>
            <CommandEmpty>No fonts found.</CommandEmpty>
            <CommandSelectGroup>
              {renderItem("", `Default · ${fontLabel("", role)}`)}
              {catalog.map(entry => renderItem(entry.family, entry.label))}
              {orphan ? renderItem(value, value) : null}
            </CommandSelectGroup>
            {installedFamilies.length > 0 ? (
              <CommandSelectGroup heading="Installed">
                {installedFamilies.map(family => renderItem(family, family))}
              </CommandSelectGroup>
            ) : null}
          </CommandList>
        </CommandSelectShell>
      </CommandSelect>
      {value !== "" ? (
        <Button
          variant="link"
          size="sm"
          className="h-auto p-0"
          data-testid={testId ? `${testId}-reset` : undefined}
          onClick={() => onChange("")}
        >
          Use default
        </Button>
      ) : null}
    </div>
  );
}
