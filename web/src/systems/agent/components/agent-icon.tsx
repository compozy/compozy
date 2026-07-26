import { KindIcon, type KindIconProps } from "@agh/ui";

import { providerIconMap } from "./provider-icon-map";

type AgentIconTone = KindIconProps["tone"];

interface AgentIconProps extends Omit<KindIconProps, "kind" | "registry"> {
  provider: string;
}

function AgentIcon({
  provider,
  "data-slot": dataSlot = "agent-icon",
  "data-provider": dataProvider,
  ...props
}: AgentIconProps) {
  const key = provider.trim().toLowerCase();
  return (
    <KindIcon
      data-slot={dataSlot}
      data-provider={dataProvider ?? key}
      kind={key}
      registry={providerIconMap}
      {...props}
    />
  );
}

export { AgentIcon };
export type { AgentIconProps, AgentIconTone };
