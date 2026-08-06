export const VERIFIED_INSTALLER_COMMAND = "curl -fsSL https://compozy.com/install.sh | sh";
export const NPM_INSTALL_COMMAND = "npm install -g @compozy/cli@beta";

export function goInstallCommand(tag: string): string {
  return `go install github.com/compozy/compozy@${tag}`;
}

export function curlPinnedInstallCommand(tag: string): string {
  return `curl -fsSL https://compozy.com/install.sh | sh -s -- --version ${tag} --dir "$HOME/.local/bin"`;
}

export function curlEnvInstallCommand(tag: string): string {
  return [
    "curl -fsSL https://compozy.com/install.sh |",
    `  COMPOZY_VERSION=${tag} COMPOZY_INSTALL_DIR="$HOME/.local/bin" COMPOZY_SKIP_BOOTSTRAP=1 sh`,
  ].join("\n");
}
