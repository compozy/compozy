import { ChevronDownIcon } from "lucide-react";
import { useId, useState, type KeyboardEvent } from "react";

import { Button } from "../button";
import { ButtonGroup } from "../button-group";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "../dropdown-menu";
import { cn } from "../../lib/utils";

type ButtonVariant = React.ComponentProps<typeof Button>["variant"];
type ButtonSize = React.ComponentProps<typeof Button>["size"];

export interface SplitButtonProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  label: React.ReactNode;
  icon?: React.ReactNode;
  onAction: () => void;
  /** Applied to both segments so the pair always reads as one control. */
  variant?: ButtonVariant;
  size?: ButtonSize;
  /**
   * The action is inoperable but still focusable, so assistive technology can
   * reach it and read `blockedReason`. A natively disabled button is skipped by
   * focus, which would make the explanation unreachable.
   */
  blocked?: boolean;
  blockedReason?: string;
  /** Hard-disables both segments — for in-flight work, not for policy. */
  disabled?: boolean;
  /** Required: no caller may ship an unlabelled menu trigger. */
  menuLabel: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  menuProps?: Omit<React.ComponentProps<typeof DropdownMenuContent>, "children">;
  /** Menu rows. Empty children collapse the control to a single button. */
  children?: React.ReactNode;
}

const SEAM_CLASS = "border-l border-l-accent-ink/20";

/**
 * A primary action paired with a menu of its alternatives.
 *
 * `ButtonGroup` already owns the collapsed inner radii and the seam, so the two
 * segments are ordinary `Button`s sharing one variant and size. The chevron
 * stays operable while the action is blocked — the alternatives are exactly what
 * an operator needs when the primary cannot run.
 */
export function SplitButton({
  label,
  icon,
  onAction,
  variant = "default",
  size = "default",
  blocked = false,
  blockedReason,
  disabled = false,
  menuLabel,
  open,
  onOpenChange,
  menuProps,
  children,
  className,
  ...props
}: SplitButtonProps) {
  const { className: menuClassName, ...restMenuProps } = menuProps ?? {};
  const reasonId = useId();
  const [internalOpen, setInternalOpen] = useState(false);
  const hasMenu = Boolean(children);
  const describedBy = blocked && blockedReason ? reasonId : undefined;
  const menuOpen = open ?? internalOpen;

  function setMenuOpen(nextOpen: boolean) {
    if (open === undefined) setInternalOpen(nextOpen);
    onOpenChange?.(nextOpen);
  }

  function handleActionKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key === "ArrowDown" && hasMenu) {
      event.preventDefault();
      setMenuOpen(true);
      return;
    }
    if ((event.key === " " || event.key === "Space") && !event.repeat) {
      // Base UI does not synthesize the native Space click in every host
      // environment. Owning it here keeps the split action keyboard-complete;
      // preventing the key default also prevents a second browser click.
      event.preventDefault();
      if (!blocked && !disabled) onAction();
    }
  }

  return (
    <ButtonGroup
      className={cn("inline-flex items-stretch", className)}
      data-blocked={blocked ? "" : undefined}
      data-disabled={disabled ? "" : undefined}
      data-slot="split-button"
      data-variant={variant}
      orientation="horizontal"
      {...props}
    >
      <Button
        aria-describedby={describedBy}
        aria-disabled={blocked || undefined}
        className="gap-1.5 whitespace-nowrap aria-disabled:pointer-events-none aria-disabled:cursor-not-allowed aria-disabled:opacity-50 [&_svg]:size-3"
        data-size={size}
        data-slot="split-button-action"
        data-variant={variant}
        disabled={disabled}
        onClick={blocked ? undefined : onAction}
        onKeyDown={handleActionKeyDown}
        size={size}
        type="button"
        variant={variant}
      >
        {icon}
        {label}
      </Button>
      {blocked && blockedReason ? (
        <span className="sr-only" id={reasonId}>
          {blockedReason}
        </span>
      ) : null}
      {hasMenu ? (
        <DropdownMenu onOpenChange={setMenuOpen} open={menuOpen}>
          <DropdownMenuTrigger
            aria-label={menuLabel}
            data-size={size}
            data-slot="split-button-trigger"
            data-variant={variant}
            render={
              <Button
                className={cn("w-8 px-0 [&_svg]:size-3", variant === "default" && SEAM_CLASS)}
                disabled={disabled}
                size={size}
                type="button"
                variant={variant}
              />
            }
          >
            <ChevronDownIcon aria-hidden="true" className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className={cn("w-max max-w-72 p-1", menuClassName)}
            {...restMenuProps}
          >
            {children}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </ButtonGroup>
  );
}
