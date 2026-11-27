import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  answerTerminalInputRequest,
  closeTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
  terminalCatalogQuery,
  terminalInputRequestsQuery,
  terminalKeys,
  terminalScope,
  useTerminalCatalogStream,
  waitTerminal,
  type TerminalInputRequest,
  type TerminalWindowActions,
} from "@/systems/terminal";

/**
 * Wires the phone Terminal tab directly to `TerminalWindowApp` without forking
 * the terminal renderer, reusing the canonical catalog query, live catalog stream,
 * and terminal process mutations.
 */
export function useHostedPhoneTerminal(workspaceId: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const [selectedTerminalId, setSelectedTerminalId] = useState<string | null>(null);
  const profile = "default";
  const scope = terminalScope(workspaceId, profile);
  const active = enabled && workspaceId.trim() !== "";

  const catalog = useQuery({
    ...terminalCatalogQuery(scope),
    enabled: active,
  });
  const inputRequests = useQuery({
    ...terminalInputRequestsQuery(scope),
    enabled: active,
  });

  useTerminalCatalogStream({ workspaceId, profileKey: profile, enabled: active });

  const closeMutation = useMutation({
    mutationFn: (terminalId: string) => closeTerminal(workspaceId, terminalId, { profile }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: terminalKeys.catalog(scope.key) });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Couldn't close terminal.");
    },
  });

  const stopMutation = useMutation({
    mutationFn: (terminalId: string) => signalTerminal(workspaceId, terminalId, "INT", { profile }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: terminalKeys.catalog(scope.key) });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Couldn't stop command.");
    },
  });

  const waitMutation = useMutation({
    mutationFn: (terminalId: string) =>
      waitTerminal(workspaceId, terminalId, { until: "exit" }, { profile }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: terminalKeys.catalog(scope.key) });
    },
  });

  const answerMutation = useMutation({
    mutationFn: ({ request, value }: { request: TerminalInputRequest; value: string }) =>
      answerTerminalInputRequest(workspaceId, request.terminal_id, request.id, value, { profile }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: terminalKeys.inputRequests(scope.key) });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Couldn't send answer.");
    },
  });

  const rejectMutation = useMutation({
    mutationFn: (request: TerminalInputRequest) =>
      rejectTerminalInputRequest(workspaceId, request.terminal_id, request.id, "dismissed", {
        profile,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: terminalKeys.inputRequests(scope.key) });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Couldn't dismiss request.");
    },
  });

  const terminals = catalog.data ?? [];
  const resolvedTerminalId =
    selectedTerminalId && terminals.some(item => item.id === selectedTerminalId)
      ? selectedTerminalId
      : (terminals[0]?.id ?? null);

  const actions: TerminalWindowActions = {
    retargetTerminal: setSelectedTerminalId,
    onCloseTerminal: terminalId =>
      closeMutation.mutateAsync(terminalId).then(
        () => true,
        () => false
      ),
    closePending: closeMutation.isPending,
    onStop: terminalId => stopMutation.mutate(terminalId),
    onWait: terminalId => waitMutation.mutate(terminalId),
    onAnswerInputRequest: (request, value) => answerMutation.mutate({ request, value }),
    onRejectInputRequest: request => rejectMutation.mutate(request),
    onSendSelection: () => undefined,
    onCopySelection: () => undefined,
    onChooseSession: () => undefined,
    onStartSession: () => undefined,
    hasActiveSession: false,
  };

  return {
    profile,
    terminals,
    selectedTerminalId: resolvedTerminalId,
    setSelectedTerminalId,
    inputRequests: inputRequests.data?.pending ?? [],
    resolvedInputRequests: inputRequests.data?.resolved ?? [],
    isPending: catalog.isPending,
    error: catalog.error,
    refetch: catalog.refetch,
    isFetchedAfterMount: catalog.isFetchedAfterMount,
    actions,
  };
}
