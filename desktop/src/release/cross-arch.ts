export function shouldDisableDifferentialDownload(
  processArchitecture: string,
  machineArchitecture: string
): boolean {
  return processArchitecture === "x64" && machineArchitecture === "arm64";
}
