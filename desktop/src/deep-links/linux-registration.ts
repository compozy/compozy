import { execFile } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { promisify } from "node:util";

export function linuxDesktopEntry(executable: string, version: string): string {
  if (/[\r\n]/u.test(executable + version)) throw new Error("Invalid desktop entry value.");
  const quoted = executable
    .replaceAll("%", "%%")
    .replace(/[\\"`$]/gu, "\\$&")
    .replaceAll("\\", "\\\\");
  return `[Desktop Entry]\nType=Application\nName=CompozyOS\nComment=CompozyOS Electron desktop shell\nExec="${quoted}" %u\nIcon=compozyos\nTerminal=false\nCategories=Development;\nMimeType=x-scheme-handler/compozyos;\nStartupWMClass=compozyos\nX-Compozy-Version=${version}\nX-AppImage-Version=${version}\n`;
}

export async function registerLinuxDesktop(
  executable: string,
  version: string,
  dataHome: string
): Promise<void> {
  const applications = join(dataHome, "applications");
  await mkdir(applications, { recursive: true });
  await writeFile(join(applications, "compozyos.desktop"), linuxDesktopEntry(executable, version), {
    mode: 0o644,
  });
  await promisify(execFile)("xdg-mime", [
    "default",
    "compozyos.desktop",
    "x-scheme-handler/compozyos",
  ]);
}
