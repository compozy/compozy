import type { LucideIcon } from "lucide-react";
import { createElement, type ReactElement } from "react";

interface IconProps {
  icon: LucideIcon;
}

export function create({ icon }: IconProps): ReactElement {
  return createElement(icon, {
    className: "w-4 h-4",
  });
}
