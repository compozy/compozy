import { useSetRuntimeMCPMode } from "../hooks/use-mcp-runtime-mutations";
import { MCPRuntimeModeSwitchView } from "./mcp-runtime-mode-switch-view";

export interface MCPRuntimeModeSwitchProps {
  /** Provider config key that owns `runtime_mcp` (`claude`, `copilot`, `qwen`). */
  provider: string;
  runtimeName: string;
  mode: string;
  label?: string;
  className?: string;
  "data-testid"?: string;
}

/**
 * The switch bound to `providers.<provider>.runtime_mcp`. It shows the pending value while the
 * write runs and reverts with the reason when it fails. Shared by the runtime group header and the
 * provider detail.
 */
export function MCPRuntimeModeSwitch({ provider, mode, ...props }: MCPRuntimeModeSwitchProps) {
  const mutation = useSetRuntimeMCPMode();
  const saved = mode !== "off";
  const checked = mutation.isPending ? mutation.variables.mode === "load" : saved;
  return (
    <MCPRuntimeModeSwitchView
      {...props}
      checked={checked}
      error={mutation.isError ? mutation.error.message : null}
      onCheckedChange={next => mutation.mutate({ provider, mode: next ? "load" : "off" })}
      pending={mutation.isPending}
    />
  );
}
