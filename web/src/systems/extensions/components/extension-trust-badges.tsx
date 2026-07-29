import { Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { extensionSourceKindLabel } from "../lib/extension-source-kind";
import type { ExtensionTrustFacts } from "../lib/extension-trust-facts";

export const EXTENSION_DEV_LABEL = "dev";
export const EXTENSION_OVERRIDES_PUBLISHED_LABEL = "overrides published";
export const EXTENSION_DIGEST_MATCHED_LABEL = "Digest matched";
export const EXTENSION_CHECKSUM_VERIFIED_LABEL = "Checksum verified";

const DIGEST_MATCHED_HINT =
  "Integrity only: the downloaded archive matches its published digest. It does not verify the publisher.";
const CHECKSUM_VERIFIED_HINT = "The catalog pins this archive's checksum for a curated entry.";

function BadgeWithHint({
  hint,
  label,
  testId,
  tone,
}: {
  hint: string;
  label: string;
  testId: string;
  tone: "info" | "success";
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Pill data-testid={testId} mono size="xs" tone={tone}>
            {label}
          </Pill>
        }
      />
      <TooltipContent>{hint}</TooltipContent>
    </Tooltip>
  );
}

/**
 * Four distinct labels, never merged: a dev overlay, the published row it shadows, archive
 * integrity, and curated checksum pinning. Only curated checksum pinning reads as verified.
 */
export function ExtensionTrustBadges({
  facts,
  showRegistryTier = true,
  showSource = false,
}: {
  facts: ExtensionTrustFacts;
  /** Disable where a dedicated registry-tier row already carries the value. */
  showRegistryTier?: boolean;
  showSource?: boolean;
}) {
  return (
    <>
      {facts.dev ? (
        <Pill data-testid="extension-dev-badge" mono size="xs" tone="accent">
          {EXTENSION_DEV_LABEL}
        </Pill>
      ) : null}
      {facts.overridesPublished ? (
        <Pill data-testid="extension-overrides-published-badge" mono size="xs" tone="warning">
          {EXTENSION_OVERRIDES_PUBLISHED_LABEL}
        </Pill>
      ) : null}
      {/* A dev overlay already says `dev`; repeating it as the source adds no information. */}
      {showSource && !(facts.dev && facts.source === EXTENSION_DEV_LABEL) ? (
        <Pill data-testid="extension-source-badge" mono size="xs">
          {extensionSourceKindLabel(facts.source)}
        </Pill>
      ) : null}
      {showRegistryTier && facts.registryTier ? (
        <Pill data-testid="extension-registry-tier-badge" mono size="xs">
          {facts.registryTier}
        </Pill>
      ) : null}
      {facts.digestMatched ? (
        <BadgeWithHint
          hint={DIGEST_MATCHED_HINT}
          label={EXTENSION_DIGEST_MATCHED_LABEL}
          testId="extension-digest-matched-badge"
          tone="info"
        />
      ) : null}
      {facts.checksumVerified ? (
        <BadgeWithHint
          hint={CHECKSUM_VERIFIED_HINT}
          label={EXTENSION_CHECKSUM_VERIFIED_LABEL}
          testId="extension-checksum-verified-badge"
          tone="success"
        />
      ) : null}
    </>
  );
}
