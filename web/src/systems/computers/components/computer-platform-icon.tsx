import type { ComponentProps } from "react";
import { Cpu, Laptop, Monitor, ServerIcon, Smartphone, Terminal } from "@compozy/ui/icons";

import { KindIcon, type KindIconSize, type KindIconTone } from "@compozy/ui";

import type {
  ComputerIconKind,
  ComputerPlatform,
  DeviceKind,
} from "../mocks/remote-computers-fixtures";

export interface ComputerPlatformIconProps extends Omit<ComponentProps<"span">, "children"> {
  platform?: ComputerPlatform;
  iconKind?: ComputerIconKind;
  deviceKind?: DeviceKind;
  size?: KindIconSize;
  tone?: KindIconTone;
}

function resolveGlyph({
  platform,
  iconKind,
  deviceKind,
}: Pick<ComputerPlatformIconProps, "platform" | "iconKind" | "deviceKind">) {
  if (deviceKind === "browser") return Smartphone;
  if (deviceKind === "cli") return Terminal;
  if (deviceKind === "desktop") return Laptop;

  if (iconKind === "laptop") return Laptop;
  if (iconKind === "desktop") return Monitor;
  if (iconKind === "gpu") return Cpu;
  if (iconKind === "windows") return Monitor;
  if (iconKind === "server") return ServerIcon;

  if (platform === "darwin_arm64" || platform === "darwin_x86_64") return Laptop;
  if (platform === "windows_x86_64") return Monitor;
  return ServerIcon;
}

/**
 * Consistent hardware/device icon across Computers list, Details header,
 * Run on picker, Account devices, and Phone browser switcher.
 */
export function ComputerPlatformIcon({
  platform,
  iconKind,
  deviceKind,
  size = "sm",
  tone = "muted",
  ...props
}: ComputerPlatformIconProps) {
  const glyph = resolveGlyph({ platform, iconKind, deviceKind });
  return <KindIcon aria-hidden="true" icon={glyph} size={size} tone={tone} {...props} />;
}
