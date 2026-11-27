import { useMemo, useState } from "react";

import type { EntityMode } from "@compozy/ui";

import type { RunOnTarget } from "@/systems/computers/components/run-on-picker";
import type {
  ComputerFixture,
  LocalComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  projectsForTarget,
  resolveAutoComputer,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import {
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "@/systems/session/lib/session-environment-target";

import { worktreesForProject } from "../run-on-fixtures";

interface UseRunOnSessionCreateInput {
  localComputer: LocalComputerFixture;
  computers: readonly ComputerFixture[];
  initialTarget: RunOnTarget;
  initialProject?: string;
  initialAgent: string;
  initialSessionName: string;
  initialEnvironment: SessionEnvironmentTarget;
  initialReconnectingIds: readonly string[];
  initialMode: EntityMode;
}

export function useRunOnSessionCreate({
  localComputer,
  computers,
  initialTarget,
  initialProject,
  initialAgent,
  initialSessionName,
  initialEnvironment,
  initialReconnectingIds,
  initialMode,
}: UseRunOnSessionCreateInput) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const [selectedAgentName, setSelectedAgentName] = useState(initialAgent);
  const [sessionName, setSessionName] = useState(initialSessionName);
  const [environmentTarget, setEnvironmentTarget] =
    useState<SessionEnvironmentTarget>(initialEnvironment);
  const [reconnectingIds, setReconnectingIds] = useState<string[]>([...initialReconnectingIds]);
  const [upgradeNotice, setUpgradeNotice] = useState<string | null>(null);

  const availableProjects = useMemo(() => projectsForTarget(target), [target]);
  const [selectedProjectName, setSelectedProjectName] = useState<string>(
    initialProject ?? availableProjects[0]?.name ?? "api"
  );

  const effectiveProject = useMemo(() => {
    const match = availableProjects.find(project => project.name === selectedProjectName);
    return match ?? availableProjects[0] ?? null;
  }, [availableProjects, selectedProjectName]);

  const worktrees = useMemo(
    () => worktreesForProject(effectiveProject?.id),
    [effectiveProject?.id]
  );

  const autoResolution = useMemo(() => {
    if (!effectiveProject) return null;
    return resolveAutoComputer(effectiveProject.name, localComputer, computers);
  }, [effectiveProject, localComputer, computers]);

  const selectedRemote = useMemo(
    () => computers.find(computer => computer.id === target) ?? null,
    [computers, target]
  );

  const isOfflineTarget = selectedRemote?.status === "offline";
  const isNoEligibleAuto = target === "auto" && !autoResolution;
  const isLocalUnavailable =
    !localComputer.localRunAgents &&
    computers.filter(c => c.enabled && c.status === "connected").length === 0;
  const isReconnectingTarget =
    selectedRemote !== null && reconnectingIds.includes(selectedRemote.id);
  const submitBlocked = isOfflineTarget || isNoEligibleAuto || isLocalUnavailable;

  const handleTargetChange = (next: RunOnTarget) => {
    setTarget(next);
    setUpgradeNotice(null);
    const nextProjects = projectsForTarget(next);
    if (nextProjects.length > 0 && !nextProjects.some(p => p.name === selectedProjectName)) {
      setSelectedProjectName(nextProjects[0]!.name);
    }
    setEnvironmentTarget(ROOT_ENVIRONMENT_TARGET);
  };

  const handleProjectChange = (nextProjectId: string) => {
    const picked = availableProjects.find(project => project.id === nextProjectId);
    if (!picked) return;
    setSelectedProjectName(picked.name);
    setEnvironmentTarget(ROOT_ENVIRONMENT_TARGET);
  };

  const handleReconnect = (computerId: string) => {
    setReconnectingIds(prev => (prev.includes(computerId) ? prev : [...prev, computerId]));
  };

  const handleUpgrade = (id: string) => {
    const found = computers.find(computer => computer.id === id);
    setUpgradeNotice(
      `${found?.name ?? "This computer"} needs Compozy Pro. Opening the Upgrade sheet…`
    );
  };

  return {
    target,
    mode,
    setMode,
    selectedAgentName,
    setSelectedAgentName,
    sessionName,
    setSessionName,
    environmentTarget,
    setEnvironmentTarget,
    reconnectingIds,
    upgradeNotice,
    availableProjects,
    effectiveProject,
    worktrees,
    autoResolution,
    selectedRemote,
    isOfflineTarget,
    isNoEligibleAuto,
    isReconnectingTarget,
    submitBlocked,
    handleTargetChange,
    handleProjectChange,
    handleReconnect,
    handleUpgrade,
  };
}
