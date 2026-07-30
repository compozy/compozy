/**
 * Distribution facts the daemon records per installation. `digestMatched` is integrity only — a
 * colocated digest sidecar proves the archive was not altered in transit and never elevates tier,
 * consent, or publisher trust. `checksumVerified` is exclusively catalog-pinned curated installs.
 */
export interface ExtensionTrustFacts {
  checksumVerified: boolean;
  dev: boolean;
  digestMatched: boolean;
  originPath: string | null;
  overridesPublished: boolean;
  registryTier: string | null;
  source: string;
}

interface ExtensionTrustEvidence {
  checksum_verified?: boolean;
  digest_matched?: boolean;
  registry_tier?: string;
}

interface ExtensionProvenanceEvidence extends ExtensionTrustEvidence {
  installed_from?: string;
}

/** Accepts either an extension projection or a standalone provenance record. */
export interface ExtensionTrustSource extends ExtensionTrustEvidence {
  dev?: boolean;
  installed_from?: string;
  origin_path?: string;
  overrides_published?: boolean;
  provenance?: ExtensionProvenanceEvidence | null;
  source?: string;
  trust?: ExtensionTrustEvidence | null;
}

export function extensionTrustFacts(extension: ExtensionTrustSource): ExtensionTrustFacts {
  const tier =
    extension.trust?.registry_tier ??
    extension.provenance?.registry_tier ??
    extension.registry_tier ??
    "";
  const installedFrom = (
    extension.provenance?.installed_from ??
    extension.installed_from ??
    extension.source ??
    ""
  )
    .trim()
    .toLowerCase();
  const curatedChecksum =
    installedFrom === "marketplace_registry" ||
    installedFrom === "marketplace" ||
    installedFrom === "curated";
  return {
    checksumVerified:
      curatedChecksum &&
      (extension.trust?.checksum_verified === true ||
        extension.provenance?.checksum_verified === true ||
        extension.checksum_verified === true),
    dev: extension.dev === true,
    digestMatched:
      extension.digest_matched === true || extension.provenance?.digest_matched === true,
    originPath: extension.origin_path?.trim() || null,
    overridesPublished: extension.overrides_published === true,
    registryTier: tier.trim() === "" ? null : tier.trim(),
    source: installedFrom || "unknown",
  };
}
