"use client";

import { Moon, Sun } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";

export interface ThemeToggleProps extends Omit<React.ComponentProps<"button">, "children"> {
  /** The theme currently painted; the toggle offers the other one. */
  resolvedTheme: "light" | "dark";
}

/**
 * The shell's light/dark switch: a quiet rail-foot icon button that shows the
 * theme it switches to — sun while dark, moon while light — and says so in its
 * accessible name. Presentational; the owner persists the choice in `onClick`.
 */
function ThemeToggle({ resolvedTheme, className, type = "button", ...props }: ThemeToggleProps) {
  const dark = resolvedTheme === "dark";
  const Icon = dark ? Sun : Moon;
  return (
    <button
      aria-label={dark ? "Switch to light mode" : "Switch to dark mode"}
      data-resolved-theme={resolvedTheme}
      data-slot="theme-toggle"
      type={type}
      className={cn(
        "grid size-rail-item shrink-0 cursor-pointer place-items-center rounded-lg text-muted",
        "transition-colors duration-base ease-out hover:bg-rail-hover hover:text-fg",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        "[&_svg]:pointer-events-none [&_svg]:size-4",
        className
      )}
      {...props}
    >
      <Icon aria-hidden="true" />
    </button>
  );
}

export { ThemeToggle };
