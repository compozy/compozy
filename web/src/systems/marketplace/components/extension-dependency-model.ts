import type { ExtensionDependency, ExtensionInstallPreview } from "@/systems/extensions";
import type { ExtensionInstallRequest } from "../types";

/** One consent decision a dependency needs before the joint install, in preview order. */
export type ExtensionDependencyGate =
  | { kind: "trust"; name: string; requiredBy: string[] }
  | { kind: "gateway"; name: string; digest: string };

const SOURCE_LABEL: Record<string, string> = {
  curated: "Curated catalog",
  git: "Git URL",
  github: "GitHub release",
  local_path: "Local path",
  marketplace: "Marketplace",
};

export function extensionDependencySourceLabel(source: string): string {
  return SOURCE_LABEL[source] ?? source;
}

/**
 * Only the recorded official or community tier is registry verification; consent an installed
 * dependency was granted earlier is not.
 */
export function isRegistryVerifiedDependency(dependency: ExtensionDependency): boolean {
  if (dependency.consent_required) return false;
  return dependency.registry_tier === "official" || dependency.registry_tier === "community";
}

/** The dependencies Add acts on; a met dependency needs nothing and stays hidden. */
export function unmetExtensionDependencies(
  preview: Pick<ExtensionInstallPreview, "dependencies">
): ExtensionDependency[] {
  return (preview.dependencies ?? []).filter(dependency => dependency.state !== "met");
}

/**
 * A dependency Add cannot install blocks it: an installed one below the required version, a source
 * with no satisfying version, or a different copy another profile or workspace uses. Nothing is
 * auto-updated.
 */
export function blockedExtensionDependencies(
  preview: Pick<ExtensionInstallPreview, "dependencies">
): ExtensionDependency[] {
  return (preview.dependencies ?? []).filter(
    dependency => extensionDependencyBlockedReason(dependency) !== null
  );
}

/**
 * The guidance a blocking dependency row shows, or null when Add can install the dependency. The
 * daemon's retry_command is the only command that can unblock it; without one, no update reaches
 * min_version and the copy must be reinstalled from a source that provides it.
 */
export function extensionDependencyBlockedReason(dependency: ExtensionDependency): string | null {
  const minVersion = dependency.min_version ?? "the required version";
  const run = dependency.retry_command;
  if (dependency.state === "installed_elsewhere") {
    const copy = `Another profile or workspace uses a different copy of ${dependency.name}.`;
    return run
      ? `${copy} Run ${run} first`
      : `${copy} Reinstall it there from a source with ${minVersion} or later first`;
  }
  if (dependency.state !== "version_unmet") return null;
  if (!dependency.installed_version) {
    return `Its source has no version of ${dependency.name} at ${minVersion} or later`;
  }
  return run
    ? `Update ${dependency.name} to ${minVersion} or later first: run ${run}`
    : `No update of ${dependency.name} reaches ${minVersion}. Reinstall it from a source with ${minVersion} or later first`;
}

function installableDependencies(preview: Pick<ExtensionInstallPreview, "dependencies">) {
  return (preview.dependencies ?? []).filter(
    dependency => dependency.state === "missing" || dependency.state === "disabled"
  );
}

/**
 * Each dependency keeps its own trust gates — unverified consent, then the gateway digest — in the
 * preview's topological order, dependencies first. There is no blanket consent.
 */
export function extensionDependencyGates(
  preview: Pick<ExtensionInstallPreview, "dependencies">
): ExtensionDependencyGate[] {
  const gates: ExtensionDependencyGate[] = [];
  for (const dependency of installableDependencies(preview)) {
    if (dependency.consent_required) {
      gates.push({ kind: "trust", name: dependency.name, requiredBy: dependency.required_by });
    }
    if (dependency.gateway_requirement_digest) {
      gates.push({
        digest: dependency.gateway_requirement_digest,
        kind: "gateway",
        name: dependency.name,
      });
    }
  }
  return gates;
}

/**
 * The joint-install fields for a preview with dependencies to install or turn on: the opt-in plus
 * the per-dependency consent the gates above confirmed, pinned to the previewed digests.
 */
export function extensionDependencyInstallFields(
  preview: Pick<ExtensionInstallPreview, "dependencies">
): Pick<ExtensionInstallRequest, "with_dependencies" | "dependencies"> {
  const installable = installableDependencies(preview);
  if (installable.length === 0) return {};
  const dependencies: NonNullable<ExtensionInstallRequest["dependencies"]> = {};
  for (const dependency of installable) {
    const consent = {
      ...(dependency.digest_sha256 ? { expected_digest: dependency.digest_sha256 } : {}),
      ...(dependency.consent_required ? { allow_unverified: true } : {}),
      ...(dependency.gateway_requirement_digest
        ? { confirm_gateway_digest: dependency.gateway_requirement_digest }
        : {}),
    };
    if (Object.keys(consent).length > 0) dependencies[dependency.name] = consent;
  }
  return {
    with_dependencies: true,
    ...(Object.keys(dependencies).length > 0 ? { dependencies } : {}),
  };
}
