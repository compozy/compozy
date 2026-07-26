import { KindIcon, providerKindIconRegistry, type KindIconProps } from "@agh/ui";

interface ProviderLogoProps extends Omit<KindIconProps, "kind" | "registry" | "size"> {
  provider: string;
}

export function ProviderLogo({
  provider,
  "data-slot": dataSlot = "provider-logo",
  "data-provider": dataProvider,
  className,
  ...props
}: ProviderLogoProps) {
  const key = provider.trim().toLowerCase();
  return (
    <KindIcon
      data-slot={dataSlot}
      data-provider={dataProvider ?? key}
      kind={key}
      registry={providerKindIconRegistry}
      size="md"
      tone="default"
      className={className}
      {...props}
    />
  );
}
