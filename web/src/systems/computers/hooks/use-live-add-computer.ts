import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  addAccountComputer,
  answerComputerSetup,
  cancelComputerSetup,
  ComputersApiError,
  fetchComputerSetupStatus,
  openComputersStream,
  startSSHComputerSetup,
} from "../adapters/computers-api";
import type { SshTargetDraft } from "../components/add-computer-ssh-picker";
import { computerRegistryKeys } from "../lib/query-keys";
import { availableComputersOptions, computerSSHHostsOptions } from "../lib/query-options";
import {
  extractLiveHostKeyPrompt,
  isSetupLinkedCommitReached,
  toLiveSshStage,
  type LiveHostKeyPromptInfo,
} from "../lib/ssh-setup-view";
import { toAvailableComputerView, toSSHHostSuggestionView } from "../lib/computer-view";
import type {
  AccountUnaddedComputerFixture,
  SshHostSuggestionFixture,
} from "../mocks/remote-computers-types";
import type {
  AddComputerMethod,
  AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";
import type { ComputerPayload, ComputerSetupStatusPayload } from "../types";

export interface AddComputerLiveController {
  method: AddComputerMethod;
  setMethod: (method: AddComputerMethod) => void;
  draft: SshTargetDraft;
  setDraft: (draft: SshTargetDraft) => void;
  sshStage: AddComputerSshStage;
  setupStatus: ComputerSetupStatusPayload | null;
  hostKeyPrompt: LiveHostKeyPromptInfo | null;
  secretCountdownSeconds: number;
  secretError: string | null;
  isAgentInitiated: boolean;
  linkedCommitReached: boolean;
  addedIds: readonly string[];
  detectedCommandComputer: AccountUnaddedComputerFixture | null;
  onStartSsh: () => void;
  onTrustHost: () => void;
  onDeclineHost: () => void;
  onSubmitSecret: (secret: string) => void;
  onConfirmCancel: () => void;
  onRetrySetup: () => void;
  onAddAccountComputer: (computer: AccountUnaddedComputerFixture) => void;
  onBackToPicker: () => void;
  onRemoveLinked: () => void;
}

const EMPTY_DRAFT: SshTargetDraft = {
  host: "",
  user: "",
  port: "22",
  name: "",
};

const SECRET_PROMPT_TIMEOUT_SECONDS = 180;

export interface UseLiveAddComputerInput {
  open: boolean;
  signedIn: boolean;
  addedComputers: readonly ComputerPayload[];
  onOpenModal?: () => void;
  onRequestRemoveById?: (computerId: string) => void;
}

export function useLiveAddComputer({
  open,
  signedIn,
  addedComputers,
  onOpenModal,
  onRequestRemoveById,
}: UseLiveAddComputerInput) {
  const queryClient = useQueryClient();

  const [method, setMethod] = useState<AddComputerMethod>("ssh");
  const [draft, setDraft] = useState<SshTargetDraft>(EMPTY_DRAFT);
  const [setupId, setSetupId] = useState<string | null>(null);
  const [setupStatus, setSetupStatus] = useState<ComputerSetupStatusPayload | null>(null);
  const [secretCountdownSeconds, setSecretCountdownSeconds] = useState(
    SECRET_PROMPT_TIMEOUT_SECONDS
  );
  const [secretError, setSecretError] = useState<string | null>(null);
  const [isAgentInitiated, setIsAgentInitiated] = useState(false);
  const [addedIds, setAddedIds] = useState<string[]>([]);
  const [baselineAvailableIds, setBaselineAvailableIds] = useState<ReadonlySet<string> | null>(
    null
  );

  const activeSetupIdRef = useRef<string | null>(null);
  activeSetupIdRef.current = setupId;

  const sshHostsQuery = useQuery(computerSSHHostsOptions(open && signedIn));
  const availableQuery = useQuery({
    ...availableComputersOptions(open && signedIn),
    refetchInterval: open && method === "command" ? 3000 : false,
  });

  const sshSuggestions = useMemo<SshHostSuggestionFixture[]>(() => {
    return (sshHostsQuery.data?.hosts ?? []).map(host =>
      toSSHHostSuggestionView(host, addedComputers)
    );
  }, [sshHostsQuery.data?.hosts, addedComputers]);

  const unaddedComputers = useMemo<AccountUnaddedComputerFixture[]>(() => {
    return (availableQuery.data?.computers ?? [])
      .filter(computer => !computer.added)
      .map(toAvailableComputerView);
  }, [availableQuery.data?.computers]);

  // Capture baseline unadded IDs when entering the "Link with a command" tab so newly linked
  // computers surface in the watcher card automatically.
  useEffect(() => {
    if (!open || method !== "command") {
      setBaselineAvailableIds(null);
      return;
    }
    if (baselineAvailableIds === null && availableQuery.data) {
      setBaselineAvailableIds(
        new Set(availableQuery.data.computers.filter(c => !c.added).map(c => c.id))
      );
    }
  }, [open, method, availableQuery.data, baselineAvailableIds]);

  const detectedCommandComputer = useMemo<AccountUnaddedComputerFixture | null>(() => {
    if (method !== "command" || baselineAvailableIds === null) return null;
    const newlyLinked = unaddedComputers.find(item => !baselineAvailableIds.has(item.id));
    return newlyLinked ?? null;
  }, [method, baselineAvailableIds, unaddedComputers]);

  const syncSetupStatus = useCallback(
    async (id: string) => {
      try {
        const next = await fetchComputerSetupStatus(id);
        if (activeSetupIdRef.current !== id) return;
        setSetupStatus(next);
        if (next.state === "done") {
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        }
      } catch {
        // Ignore transient fetch failures; SSE stream or retry will refresh.
      }
    },
    [queryClient]
  );

  // Listen to `/api/computers/stream` for `computer.setup` events — including agent-initiated
  // secret handoffs (`needs_operator_secret`) even when the dialog starts closed.
  useEffect(() => {
    if (!signedIn) return;
    return openComputersStream({
      onComputerSetup: change => {
        const incomingSetupId = change.setup_id;
        if (!incomingSetupId) return;

        // Agent-initiated secret prompt handoff (`_uiux.md` U2): auto-open Add computer dialog.
        if (
          change.state === "needs_operator_secret" &&
          incomingSetupId !== activeSetupIdRef.current
        ) {
          setSetupId(incomingSetupId);
          setIsAgentInitiated(true);
          setMethod("ssh");
          setSecretCountdownSeconds(SECRET_PROMPT_TIMEOUT_SECONDS);
          setSecretError(null);
          setSetupStatus({
            setup_id: incomingSetupId,
            computer_id: change.computer_id ?? null,
            state: change.state,
            step: change.step ?? "connect",
            prompt: change.prompt ?? null,
            prompt_kind: change.prompt?.kind ?? "secret",
            error: change.error ?? null,
          });
          if (change.prompt?.host) {
            setDraft(prev => ({
              ...prev,
              host: change.prompt?.host || prev.host,
              name: prev.name || change.prompt?.host || "",
            }));
          }
          onOpenModal?.();
          return;
        }

        if (incomingSetupId !== activeSetupIdRef.current) return;

        setSetupStatus(prev => ({
          setup_id: incomingSetupId,
          computer_id: change.computer_id ?? prev?.computer_id ?? null,
          state: change.state ?? prev?.state ?? "running",
          step: change.step ?? prev?.step ?? "admit",
          prompt: change.prompt !== undefined ? change.prompt : (prev?.prompt ?? null),
          prompt_kind: change.prompt?.kind ?? prev?.prompt_kind,
          error: change.error !== undefined ? change.error : (prev?.error ?? null),
        }));

        if (change.state === "needs_operator_secret") {
          setSecretCountdownSeconds(SECRET_PROMPT_TIMEOUT_SECONDS);
        }

        if (change.state === "done") {
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        }
      },
      onReset: () => {
        if (activeSetupIdRef.current) {
          void syncSetupStatus(activeSetupIdRef.current);
        }
      },
    });
  }, [signedIn, onOpenModal, queryClient, syncSetupStatus]);

  // Poll setup status gently while actively running in case SSE frames are coalesced/missed.
  const sshStage = toLiveSshStage(setupStatus, { isAgentInitiated });
  useEffect(() => {
    if (!open || !setupId) return;
    if (
      sshStage !== "running" &&
      sshStage !== "trust_host" &&
      sshStage !== "password" &&
      sshStage !== "agent_secret"
    ) {
      return;
    }
    const timer = window.setInterval(() => {
      void syncSetupStatus(setupId);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [open, setupId, sshStage, syncSetupStatus]);

  // Countdown timer for active secret prompts.
  useEffect(() => {
    if (sshStage !== "password" && sshStage !== "agent_secret") return;
    const timer = window.setInterval(() => {
      setSecretCountdownSeconds(prev => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [sshStage]);

  const resetSetup = useCallback(() => {
    setSetupId(null);
    setSetupStatus(null);
    setSecretError(null);
    setIsAgentInitiated(false);
    setSecretCountdownSeconds(SECRET_PROMPT_TIMEOUT_SECONDS);
  }, []);

  const handleStartSsh = useCallback(async () => {
    const trimmedHost = draft.host.trim();
    if (!trimmedHost) return;
    const matched = sshSuggestions.find(
      item => item.alias === trimmedHost || item.hostname === trimmedHost
    );
    const parsedPort = Number.parseInt(draft.port || String(matched?.port ?? 22), 10);
    setSecretError(null);
    setIsAgentInitiated(false);
    setSetupStatus({
      setup_id: "pending",
      computer_id: null,
      state: "running",
      step: "admit",
      prompt: null,
      error: null,
    });
    try {
      const started = await startSSHComputerSetup({
        host: trimmedHost,
        user: draft.user.trim() || matched?.user || undefined,
        port: Number.isFinite(parsedPort) && parsedPort > 0 ? parsedPort : undefined,
        name: draft.name.trim() || trimmedHost,
      });
      setSetupId(started.setup_id);
      setSetupStatus({
        setup_id: started.setup_id,
        computer_id: started.computer_id,
        state: "running",
        step: "admit",
        prompt: null,
        error: null,
      });
      void syncSetupStatus(started.setup_id);
    } catch (err) {
      if (err instanceof ComputersApiError) {
        setSetupStatus({
          setup_id: "error",
          computer_id: null,
          state: "failed",
          step: err.code === "plan_limit_reached" ? "admit" : "connect",
          prompt: null,
          error: {
            code: err.code,
            error: err.message,
            details: err.details,
          },
        });
        return;
      }
      setSetupStatus({
        setup_id: "error",
        computer_id: null,
        state: "failed",
        step: "connect",
        prompt: null,
        error: {
          error: err instanceof Error ? err.message : "Failed to start SSH setup.",
        },
      });
    }
  }, [draft, sshSuggestions, syncSetupStatus]);

  const handleTrustHost = useCallback(async () => {
    if (!setupId) return;
    try {
      const next = await answerComputerSetup(setupId, { kind: "host_key", accept: true });
      setSetupStatus(next);
      toast.success(`Trusted ${draft.name || draft.host} and continued setup.`);
      if (next.state === "done") {
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to trust host key.");
    }
  }, [setupId, draft.name, draft.host, queryClient]);

  const handleDeclineHost = useCallback(async () => {
    if (!setupId) {
      resetSetup();
      return;
    }
    try {
      await answerComputerSetup(setupId, { kind: "host_key", accept: false });
    } catch {
      // Ignore error on decline; reset local dialog view.
    }
    resetSetup();
    toast.message("Setup cancelled. Host key was not trusted.");
  }, [setupId, resetSetup]);

  const handleSubmitSecret = useCallback(
    async (secret: string) => {
      if (!setupId) return;
      setSecretError(null);
      try {
        const next = await answerComputerSetup(setupId, { kind: "secret", value: secret });
        setSetupStatus(next);
        if (next.state === "done") {
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        }
      } catch (err) {
        setSecretError(err instanceof Error ? err.message : "SSH authentication failed.");
      }
    },
    [setupId, queryClient]
  );

  const handleConfirmCancel = useCallback(async () => {
    if (setupId && setupId !== "pending" && setupId !== "error") {
      try {
        await cancelComputerSetup(setupId);
      } catch (err) {
        if (err instanceof ComputersApiError && err.code === "computer_setup_linked") {
          void syncSetupStatus(setupId);
          toast.message("This computer is already linked. Remove it instead.");
          return;
        }
      }
    }
    resetSetup();
    toast.message("Setup cancelled.");
  }, [setupId, resetSetup, syncSetupStatus]);

  const handleAddAccountComputer = useCallback(
    async (computer: AccountUnaddedComputerFixture) => {
      try {
        await addAccountComputer(computer.id, computer.name);
        setAddedIds(current =>
          current.includes(computer.id) ? current : [...current, computer.id]
        );
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        toast.success(`Added ${computer.name} to your computers.`);
      } catch (err) {
        toast.error(err instanceof Error ? err.message : `Failed to add ${computer.name}.`);
      }
    },
    [queryClient]
  );

  const handleRemoveLinked = useCallback(() => {
    const computerId = setupStatus?.computer_id;
    if (computerId && onRequestRemoveById) {
      onRequestRemoveById(computerId);
    }
  }, [setupStatus?.computer_id, onRequestRemoveById]);

  const hostKeyPrompt = extractLiveHostKeyPrompt(setupStatus, draft.host, draft.port);
  const linkedCommitReached = isSetupLinkedCommitReached(setupStatus);

  const controller: AddComputerLiveController = {
    method,
    setMethod,
    draft,
    setDraft,
    sshStage,
    setupStatus,
    hostKeyPrompt,
    secretCountdownSeconds,
    secretError,
    isAgentInitiated,
    linkedCommitReached,
    addedIds,
    detectedCommandComputer,
    onStartSsh: () => void handleStartSsh(),
    onTrustHost: () => void handleTrustHost(),
    onDeclineHost: () => void handleDeclineHost(),
    onSubmitSecret: secret => void handleSubmitSecret(secret),
    onConfirmCancel: () => void handleConfirmCancel(),
    onRetrySetup: () => void handleStartSsh(),
    onAddAccountComputer: item => void handleAddAccountComputer(item),
    onBackToPicker: resetSetup,
    onRemoveLinked: handleRemoveLinked,
  };

  return {
    sshSuggestions,
    unaddedComputers,
    availableError: availableQuery.error,
    controller,
    resetSetup,
  };
}
