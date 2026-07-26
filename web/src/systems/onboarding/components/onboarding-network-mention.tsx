import { cn } from "@agh/ui";
import type { ComponentProps } from "react";

/**
 * Informative Network mention for onboarding (UT-059).
 * Visit/complete/dismiss must not mutate network settings.
 */
export type OnboardingNetworkMentionProps = Omit<ComponentProps<"p">, "children">;

export function OnboardingNetworkMention({ className, ...props }: OnboardingNetworkMentionProps) {
  return (
    <p
      {...props}
      className={cn("text-sm text-muted", className)}
      data-testid="onboarding-network-mention"
    >
      Explore <span className="font-medium text-fg">Network</span> after setup. Finishing setup does
      not enable coordination or change Network settings.
    </p>
  );
}
