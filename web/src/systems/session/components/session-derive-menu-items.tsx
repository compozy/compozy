import { ArrowRightLeft, GitFork } from "lucide-react";

import { DropdownMenuItem } from "@compozy/ui";

/** Continue and Fork items; each opens its dialog once the menu finished closing. */
export function SessionDeriveMenuItems({
  archived,
  disabled,
  onContinue,
  onFork,
  onSelect,
}: {
  archived: boolean;
  disabled: boolean;
  onContinue?: () => void;
  onFork?: () => void;
  onSelect: (action: () => void) => void;
}) {
  if (archived) return null;
  return (
    <>
      {onContinue ? (
        <DropdownMenuItem
          data-testid="continue-menu-item"
          disabled={disabled}
          onClick={() => onSelect(onContinue)}
        >
          <ArrowRightLeft className="size-3" />
          Continue with another agent…
        </DropdownMenuItem>
      ) : null}
      {onFork ? (
        <DropdownMenuItem
          data-testid="fork-menu-item"
          disabled={disabled}
          onClick={() => onSelect(onFork)}
        >
          <GitFork className="size-3" />
          Fork session…
        </DropdownMenuItem>
      ) : null}
    </>
  );
}
