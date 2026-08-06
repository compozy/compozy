export const INSTALL_VERSION_PLACEHOLDER = "__COMPOZY_PINNED_VERSION__";

const RELEASE_TAG_PATTERN = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;

/**
 * Bakes an explicit release tag into the installer template. The served
 * script always pins a concrete tag — clients never resolve "latest"
 * themselves, which keeps the Sigstore provenance posture intact.
 */
export function renderInstallScript(template: string, tag: string): string {
  if (!RELEASE_TAG_PATTERN.test(tag)) {
    throw new Error(`install script release tag must be a v-prefixed semver tag, got: ${tag}`);
  }
  if (!template.includes(INSTALL_VERSION_PLACEHOLDER)) {
    throw new Error(
      `install script template is missing placeholder ${INSTALL_VERSION_PLACEHOLDER}`
    );
  }
  return template.replaceAll(INSTALL_VERSION_PLACEHOLDER, tag);
}
