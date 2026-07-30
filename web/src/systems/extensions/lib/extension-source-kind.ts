/** Distribution source kinds the daemon records in provenance (`installed_from`). */
const SOURCE_KIND_LABEL: Record<string, string> = {
  git_url: "Git URL",
  github: "GitHub release",
  local_path: "Local path",
  marketplace_registry: "Curated catalog",
};

export function extensionSourceKindLabel(installedFrom: string | null | undefined): string {
  const kind = installedFrom?.trim() ?? "";
  if (kind === "") return "—";
  return SOURCE_KIND_LABEL[kind.toLowerCase()] ?? kind;
}
