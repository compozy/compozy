import { DynamicCodeBlock } from "fumadocs-ui/components/dynamic-codeblock";
import { getCurrentRelease } from "@/lib/release/current-release";
import {
  curlEnvInstallCommand,
  curlPinnedInstallCommand,
  goInstallCommand,
} from "@/lib/release/install-commands";

const COMMAND_BUILDERS = {
  "curl-pinned": curlPinnedInstallCommand,
  "curl-env": curlEnvInstallCommand,
  go: goInstallCommand,
} as const;

export type InstallCommandMethod = keyof typeof COMMAND_BUILDERS;

export async function InstallCommand({ method }: { method: InstallCommandMethod }) {
  const release = await getCurrentRelease();
  return <DynamicCodeBlock lang="bash" code={COMMAND_BUILDERS[method](release.tag)} />;
}

export async function CurrentReleaseTag() {
  const release = await getCurrentRelease();
  return <code>{release.tag}</code>;
}
