/**
 * Viewport of the prototype canvas: a device segment (desktop, tablet, phone, fit to the area)
 * and a menu with every preset width of each device. Custom widths come from dragging the canvas
 * edges (`canvas.tsx`); all of them live in `?w=`.
 */
import { ChevronDownIcon, MonitorIcon, ScanIcon, SmartphoneIcon, TabletIcon } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  PillGroup,
} from "@compozy/ui";

import { deviceOf, devices, updateView, type Device, type Viewport } from "#gallery/shell/view";

const icons = { desktop: MonitorIcon, tablet: TabletIcon, phone: SmartphoneIcon } as const;

const deviceItems = [
  ...devices.map(item => {
    const Icon = icons[item.id];
    return {
      value: item.id as Device | "fit",
      label: (
        <Icon aria-label={`${item.label} · ${item.widths[0]?.width} px`} className="size-3.5" />
      ),
      testId: `viewport-${item.id}`,
    };
  }),
  {
    value: "fit" as const,
    label: <ScanIcon aria-label="Fit to the area" className="size-3.5" />,
    testId: "viewport-fit",
  },
];

function choose(value: Device | "fit") {
  if (value === "fit") updateView({ width: "fit" });
  else updateView({ width: devices.find(item => item.id === value)?.widths[0]?.width ?? "fit" });
}

export function ViewportControl({ width }: { width: Viewport }) {
  return (
    <div className="flex items-center gap-0.5" data-testid="viewport">
      <PillGroup
        aria-label="Device"
        items={deviceItems}
        onChange={choose}
        size="sm"
        value={deviceOf(width) ?? "fit"}
      />
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              aria-label="Exact width"
              className="font-mono"
              data-testid="viewport-width"
              size="sm"
              variant="quiet"
            />
          }
        >
          {width === "fit" ? "auto" : `${width} px`}
          <ChevronDownIcon aria-hidden="true" className="size-3" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-60">
          <DropdownMenuRadioGroup
            onValueChange={(value: string) =>
              updateView({ width: value === "fit" ? "fit" : Number(value) })
            }
            value={String(width)}
          >
            {devices.map((item, index) => (
              <DropdownMenuGroup key={item.id}>
                {index > 0 ? <DropdownMenuSeparator /> : null}
                <DropdownMenuLabel>{item.label}</DropdownMenuLabel>
                {[...item.widths]
                  .sort((a, b) => b.width - a.width)
                  .map(preset => (
                    <DropdownMenuRadioItem key={preset.width} value={String(preset.width)}>
                      <span className="flex-1">{preset.label}</span>
                      <span className="font-mono text-mono-id tracking-mono-id text-subtle">
                        {preset.width}
                      </span>
                    </DropdownMenuRadioItem>
                  ))}
              </DropdownMenuGroup>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuRadioItem value="fit">Fit to the area</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuLabel>Drag the canvas edges for any width.</DropdownMenuLabel>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
