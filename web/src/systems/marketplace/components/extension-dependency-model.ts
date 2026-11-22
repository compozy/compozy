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

/** An installed dependency below the required version blocks Add: nothing is auto-updated. */
export function versionUnmetDependencies(
  preview: Pick<ExtensionInstallPreview, "dependencies">
): ExtensionDependency[] {
  return (preview.dependencies ?? []).filter(dependency => dependency.state === "version_unmet");
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
