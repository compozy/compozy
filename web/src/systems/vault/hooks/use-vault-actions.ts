import { useMutation, useQueryClient } from "@tanstack/react-query";

import { deleteVaultSecret, putVaultSecret } from "../adapters/vault-api";
import { vaultKeys } from "../lib/query-keys";
import type { PutVaultSecretRequest } from "../types";

function invalidateVaultQueries(queryClient: ReturnType<typeof useQueryClient>, ref?: string) {
  const tasks = [queryClient.invalidateQueries({ queryKey: vaultKeys.all })];
  if (ref) {
    tasks.push(queryClient.invalidateQueries({ queryKey: vaultKeys.detail(ref) }));
  }
  return Promise.all(tasks);
}

export function usePutVaultSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: PutVaultSecretRequest) => putVaultSecret(body),
    onSettled: (result, _error, variables) =>
      invalidateVaultQueries(queryClient, result?.ref ?? variables.ref),
  });
}

export function useDeleteVaultSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (ref: string) => deleteVaultSecret(ref),
    onSettled: (_result, _error, ref) => invalidateVaultQueries(queryClient, ref),
  });
}
