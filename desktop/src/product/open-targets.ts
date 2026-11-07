/** How an editor's CLI takes a position; `none` opens the path without one. */
type LineStyle = "vscode" | "jetbrains" | "colon" | "none";

interface OpenTargetEntry {
  id: string;
  label: string;
  /** CLI looked up on the login-shell PATH (`.exe` on Windows). */
  binary: string;
  /** macOS bundle identifier, used when the CLI is not on PATH. */
  bundle: string;
  /** Windows GUI executable one directory above the CLI shim, when the shim is a batch file. */
  windowsExe?: string;
  line: LineStyle;
}

/** The built-in Open in catalog (file-explorer spec §3a). Argv is fixed per entry. */
export const OPEN_TARGETS: readonly OpenTargetEntry[] = [
  {
    id: "cursor",
    label: "Cursor",
    binary: "cursor",
    bundle: "com.todesktop.230313mzl4w4u92",
    windowsExe: "Cursor.exe",
    line: "vscode",
  },
  {
    id: "vscode",
    label: "VS Code",
    binary: "code",
    bundle: "com.microsoft.VSCode",
    windowsExe: "Code.exe",
    line: "vscode",
  },
  {
    id: "vscode-insiders",
    label: "VS Code Insiders",
    binary: "code-insiders",
    bundle: "com.microsoft.VSCodeInsiders",
    windowsExe: "Code - Insiders.exe",
    line: "vscode",
  },
  {
    id: "windsurf",
    label: "Windsurf",
    binary: "windsurf",
    bundle: "com.exafunction.windsurf",
    windowsExe: "Windsurf.exe",
    line: "vscode",
  },
  { id: "zed", label: "Zed", binary: "zed", bundle: "dev.zed.Zed", line: "colon" },
  {
    id: "sublime",
    label: "Sublime Text",
    binary: "subl",
    bundle: "com.sublimetext.4",
    line: "colon",
  },
  {
    id: "idea",
    label: "IntelliJ IDEA",
    binary: "idea",
    bundle: "com.jetbrains.intellij",
    line: "jetbrains",
  },
  {
    id: "webstorm",
    label: "WebStorm",
    binary: "webstorm",
    bundle: "com.jetbrains.WebStorm",
    line: "jetbrains",
  },
  {
    id: "goland",
    label: "GoLand",
    binary: "goland",
    bundle: "com.jetbrains.goland",
    line: "jetbrains",
  },
  {
    id: "pycharm",
    label: "PyCharm",
    binary: "pycharm",
    bundle: "com.jetbrains.pycharm",
    line: "jetbrains",
  },
  { id: "xcode", label: "Xcode", binary: "xed", bundle: "com.apple.dt.Xcode", line: "jetbrains" },
  {
    id: "android-studio",
    label: "Android Studio",
    binary: "studio",
    bundle: "com.google.android.studio",
    line: "jetbrains",
  },
  { id: "fleet", label: "Fleet", binary: "fleet", bundle: "com.jetbrains.fleet", line: "none" },
];

export function openTargetEntry(id: string): OpenTargetEntry | undefined {
  return OPEN_TARGETS.find(target => target.id === id);
}

/** Whether the target's CLI accepts a line; directories never pass one. */
export function targetSupportsLine(id: string): boolean {
  return openTargetEntry(id)?.line !== "none";
}

export function targetTemplate(id: string, withLine: boolean): string[] {
  const style = openTargetEntry(id)?.line ?? "none";
  if (!withLine || style === "none") return ["{path}"];
  if (style === "jetbrains") return ["--line", "{line}", "{path}"];
  if (style === "vscode") return ["-g", "{path}:{line}:{col}"];
  return ["{path}:{line}:{col}"];
}
