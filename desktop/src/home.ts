import { homedir } from "node:os";
import { isAbsolute, join, resolve } from "node:path";

export interface DesktopPaths {
  readonly home: string;
  readonly appRecord: string;
  readonly appSocket: string;
  readonly appToken: string;
  readonly bootstrapLog: string;
  readonly desktopLog: string;
  readonly logs: string;
  readonly operation: string;
  readonly runtime: string;
  readonly startupMarker: string;
  readonly windowState: string;
}

export function runtimeExecutableName(platform = process.platform): string {
  return platform === "win32" ? "compozy.exe" : "compozy";
}

export function resolveDesktopPaths(environment: NodeJS.ProcessEnv = process.env): DesktopPaths {
  const configured = environment.COMPOZY_HOME?.trim();
  const home = configured
    ? isAbsolute(configured)
      ? resolve(configured)
      : resolve(process.cwd(), configured)
    : join(homedir(), ".compozy");
  const logs = join(home, "logs");
  return {
    home,
    appRecord: join(home, "app.json"),
    appSocket: join(home, "app.sock"),
    appToken: join(home, "app.token"),
    bootstrapLog: join(logs, "desktop-bootstrap.jsonl"),
    desktopLog: join(logs, "desktop.log"),
    logs,
    operation: join(home, "update-operation.json"),
    runtime: join(home, "bin", runtimeExecutableName()),
    startupMarker: join(home, "desktop-startup.json"),
    windowState: join(home, "desktop-window.json"),
  };
}
