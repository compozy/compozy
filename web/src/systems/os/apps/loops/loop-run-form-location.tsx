import { AlertCircle, Repeat2 } from "lucide-react";
import { useNavigate } from "@tanstack/react-router";

import { Empty, Spinner, useTopbarSlot } from "@agh/ui";
import { LoopRunForm, useLoop, useLoopConfig } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

/**
 * Run-form entry for a Loop (design §4.3): the auto-generated typed input form, the
 * Advanced per-run overrides, the live contract preview, and Dry run / Run. On a
 * successful Run the started run's id routes to its live run page.
 */
export function LoopRunFormLocation({ name }: { name: string }) {
  const navigate = useNavigate();
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const loopQuery = useLoop(workspaceId, name, workspaceId !== "");
  const configQuery = useLoopConfig(workspaceId, name, workspaceId !== "");
  const openLoops = () => void navigate({ to: "/loops" });
  const openLoop = () => void navigate({ to: "/loops/$name", params: { name } });
  useTopbarSlot({
    onBack: openLoop,
    crumbs: [
      { id: "loops", label: "Loops", onSelect: openLoops },
      { id: "loop", label: name, onSelect: openLoop },
    ],
    crumb: "Run",
  });

  if (workspaceId === "") {
    return (
      <RunFormState
        description="Select a workspace to run this Loop."
        testId="loop-run-form-no-workspace"
        title="No workspace selected"
      />
    );
  }
  if (loopQuery.isLoading || configQuery.isLoading) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center"
        data-testid="loop-run-form-loading"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }
  if (loopQuery.error || configQuery.error || !loopQuery.data) {
    return (
      <RunFormState
        description={
          loopQuery.error?.message ?? configQuery.error?.message ?? `Loop ${name} not found.`
        }
        icon={AlertCircle}
        testId="loop-run-form-error"
        title="Unable to load loop"
      />
    );
  }

  if (!configQuery.effectiveConfig) {
    return (
      <RunFormState
        description="The daemon did not return effective loop configuration."
        icon={AlertCircle}
        testId="loop-run-form-effective-error"
        title="Unable to load loop configuration"
      />
    );
  }

  return (
    <LoopRunForm
      key={loopQuery.data.name}
      workspaceId={workspaceId}
      loop={loopQuery.data}
      effectiveConfig={configQuery.effectiveConfig}
      onCancel={openLoop}
      onRunStarted={runId => navigate({ to: "/loop-runs/$runId", params: { runId } })}
    />
  );
}

interface RunFormStateProps {
  title: string;
  description: string;
  testId: string;
  icon?: typeof Repeat2;
}

function RunFormState({ title, description, testId, icon = Repeat2 }: RunFormStateProps) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
      data-testid={testId}
    >
      <Empty className="max-w-md" description={description} icon={icon} title={title} />
    </div>
  );
}
