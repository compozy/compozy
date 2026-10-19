import type * as React from "react";
import { defaultRehypePlugins, defaultRemarkPlugins, type Streamdown } from "streamdown";

type PluginList = NonNullable<React.ComponentProps<typeof Streamdown>["rehypePlugins"]>;

declare const safePluginsBrand: unique symbol;

/** Parser plugin lists that always start with streamdown's safe-mode defaults. */
export interface SafeMarkdownPlugins {
  readonly remark: PluginList;
  readonly rehype: PluginList;
  readonly [safePluginsBrand]: true;
}

/**
 * Appends trusted plugins after the safe-mode defaults (sanitize and harden
 * included), so an extension can add nodes but never skip the sanitizer.
 * Appended rehype plugins see the sanitized tree and must emit only markup
 * they own. Call once at module scope: streamdown caches by identity.
 */
export function extendSafeMarkdownPlugins(extra: {
  remark?: PluginList;
  rehype?: PluginList;
}): SafeMarkdownPlugins {
  return {
    remark: [...Object.values(defaultRemarkPlugins), ...(extra.remark ?? [])],
    rehype: [...Object.values(defaultRehypePlugins), ...(extra.rehype ?? [])],
  } as unknown as SafeMarkdownPlugins;
}
