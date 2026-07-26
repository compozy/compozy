import { Search, Users, UserRound } from "lucide-react";
import type { ComponentProps } from "react";

import { cn, Pill, RadioCard } from "@agh/ui";

import type { NetworkFanoutPolicy } from "../types";

interface FanoutOption {
  value: NetworkFanoutPolicy;
  title: string;
  description: string;
  icon: typeof Search;
}

const FANOUT_OPTIONS: ReadonlyArray<FanoutOption> = [
  {
    value: "capability_match",
    title: "Best match",
    description: "Routes to the member whose capabilities match.",
    icon: Search,
  },
  {
    value: "coordinator",
    title: "Coordinator",
    description: "One member triages everything and delegates.",
    icon: UserRound,
  },
  {
    value: "all_members",
    title: "Everyone",
    description: "Every member receives every message.",
    icon: Users,
  },
];

interface ChannelFanoutCardsProps extends Omit<ComponentProps<"div">, "onChange"> {
  value: NetworkFanoutPolicy;
  onChange: (next: NetworkFanoutPolicy) => void;
  /**
   * Policies the surface cannot write. Create leaves `coordinator` here because
   * `coordinator_peer_id` is required for that policy and channel peers only
   * exist once the member sessions have been provisioned.
   */
  unavailable?: ReadonlyArray<NetworkFanoutPolicy>;
  /** Why an unavailable policy is out of reach on this surface. */
  unavailableHint?: string;
  disabled?: boolean;
  testIdPrefix: string;
}

/**
 * Fanout policy picker shared by channel create and channel edit.
 *
 * Consequence-bearing choice with three options, so it is a `RadioCard` grid —
 * selected state stays neutral glaze plus rim, never accent.
 */
function ChannelFanoutCards({
  value,
  onChange,
  unavailable = [],
  unavailableHint,
  disabled = false,
  testIdPrefix,
  className,
  ...props
}: ChannelFanoutCardsProps) {
  const hasUnavailable = unavailable.length > 0 && Boolean(unavailableHint);
  return (
    <div {...props} className={cn("flex flex-col gap-2", className)}>
      <div
        aria-label="Fanout policy"
        className="grid grid-cols-1 gap-2 sm:grid-cols-3"
        data-testid={`${testIdPrefix}-fanout-group`}
        role="radiogroup"
      >
        {FANOUT_OPTIONS.map(option => {
          const isUnavailable = unavailable.includes(option.value);
          return (
            <RadioCard
              className="disabled:opacity-50"
              data-testid={`${testIdPrefix}-fanout-${option.value}`}
              // The contract value stacks under the description rather than beside
              // the title: an inline badge truncates the title away at three-up.
              description={
                <>
                  {option.description}
                  <Pill className="mt-1.5 flex w-fit" mono size="xs">
                    {option.value}
                  </Pill>
                </>
              }
              disabled={disabled || isUnavailable}
              icon={option.icon}
              key={option.value}
              onSelect={() => onChange(option.value)}
              selected={value === option.value}
              title={option.title}
              titleClassName="min-w-0 flex-1 truncate"
            />
          );
        })}
      </div>
      {hasUnavailable ? (
        <p className="text-form-hint text-subtle" data-testid={`${testIdPrefix}-fanout-hint`}>
          {unavailableHint}
        </p>
      ) : null}
    </div>
  );
}

export { ChannelFanoutCards };
