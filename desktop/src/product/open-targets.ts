export const OPEN_TARGETS = [
  { id: "vscode", label: "VS Code", binary: "code", bundle: "com.microsoft.VSCode" },
  { id: "cursor", label: "Cursor", binary: "cursor", bundle: "com.todesktop.230313mzl4w4u92" },
  { id: "zed", label: "Zed", binary: "zed", bundle: "dev.zed.Zed" },
  { id: "sublime", label: "Sublime Text", binary: "subl", bundle: "com.sublimetext.4" },
  { id: "idea", label: "IntelliJ IDEA", binary: "idea", bundle: "com.jetbrains.intellij" },
  { id: "webstorm", label: "WebStorm", binary: "webstorm", bundle: "com.jetbrains.WebStorm" },
  { id: "goland", label: "GoLand", binary: "goland", bundle: "com.jetbrains.goland" },
  { id: "xcode", label: "Xcode", binary: "xed", bundle: "com.apple.dt.Xcode" },
] as const;

export function targetTemplate(id: string, withLine: boolean): string[] {
  if (!withLine) return ["{path}"];
  if (["idea", "webstorm", "goland", "xcode"].includes(id)) return ["--line", "{line}", "{path}"];
  if (id === "vscode" || id === "cursor") return ["-g", "{path}:{line}:{col}"];
  return ["{path}:{line}:{col}"];
}
