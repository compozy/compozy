/** `<SdkResource id="…" />` in a generated stub: the resource's methods from the SDK reference data. */
import { SdkLangBar } from "@/components/sdk/sdk-lang-bar";
import { SdkResourceBody } from "@/components/sdk/sdk-resource-body";
import { sdkResourceById } from "@/lib/sdk-reference";

export function SdkResource({ id }: { id: string }) {
  const resource = sdkResourceById(id);
  if (!resource) {
    throw new Error(`<SdkResource id="${id}"> names no resource; run \`bun run generate:sdk\`.`);
  }
  return (
    <div className="not-prose">
      <SdkLangBar />
      <SdkResourceBody resource={resource} />
    </div>
  );
}
