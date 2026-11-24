import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { toast } from "@compozy/ui";

import { evictRemoteComputerQueries } from "@/lib/computer-scope";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import type { CreatedAccountTokenFixture } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import type { AccountTokenFixture } from "@/systems/computers/mocks/remote-computers-types";

import {
  AccountApiError,
  createAccountToken,
  deleteAccount,
  dismissAccountSignInNotices,
  fetchAccountStatus,
  patchAccountEmail,
  revokeAccountToken,
  type AccountStatusPayload,
} from "../adapters/account-api";
import { toAccountTokenView } from "../lib/account-view";
import { accountKeys } from "../lib/query-keys";
import { accountTokensOptions } from "../lib/query-options";

export interface UseAccountLifecycleMutationsInput {
  isSession: boolean;
  email: string;
  gate: string | undefined;
}

export function useAccountLifecycleMutations({
  isSession,
  email,
  gate,
}: UseAccountLifecycleMutationsInput) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const tokensQuery = useQuery(accountTokensOptions(isSession));

  const [createTokenOpen, setCreateTokenOpen] = useState(false);
  const [createTokenError, setCreateTokenError] = useState<string | null>(null);
  const [createdToken, setCreatedToken] = useState<CreatedAccountTokenFixture | null>(null);
  const [revokingTokenId, setRevokingTokenId] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const tokens = useMemo<AccountTokenFixture[]>(
    () => (tokensQuery.data?.tokens ?? []).map(toAccountTokenView),
    [tokensQuery.data?.tokens]
  );

  const emailMutation = useMutation({
    mutationFn: (marketingOptIn: boolean) =>
      patchAccountEmail({ marketing_opt_in: marketingOptIn, source: "settings" }),
    onSuccess: updated => {
      queryClient.setQueryData(accountKeys.status(), updated);
      toast.success(
        updated.marketing_opt_in
          ? "Subscribed to product news and offers."
          : "Unsubscribed from product news and offers."
      );
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update email preference.");
    },
  });

  const createTokenMutation = useMutation({
    mutationFn: (name: string) => createAccountToken(name),
    onSuccess: created => {
      setCreateTokenOpen(false);
      setCreateTokenError(null);
      setCreatedToken({ id: created.id, name: created.name, token: created.token });
      void queryClient.invalidateQueries({ queryKey: accountKeys.tokens() });
    },
    onError: err => {
      setCreateTokenError(err instanceof Error ? err.message : "Failed to create account token.");
    },
  });

  const revokeTokenMutation = useMutation({
    mutationFn: (token: AccountTokenFixture) => revokeAccountToken(token.id),
    onMutate: token => setRevokingTokenId(token.id),
    onSettled: () => setRevokingTokenId(null),
    onSuccess: (_data, token) => {
      void queryClient.invalidateQueries({ queryKey: accountKeys.tokens() });
      toast.success(`Revoked account token "${token.name}".`);
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to revoke account token.");
    },
  });

  const dismissNoticesMutation = useMutation({
    mutationFn: () => dismissAccountSignInNotices(),
    onSuccess: () => {
      queryClient.setQueryData<AccountStatusPayload | undefined>(accountKeys.status(), current =>
        current ? { ...current, recent_sign_ins: [] } : current
      );
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to dismiss sign-in notices.");
    },
  });

  const deleteAccountMutation = useMutation({
    mutationFn: () => deleteAccount(email),
    onSuccess: () => {
      setDeleteOpen(false);
      setDeleteError(null);
      evictRemoteComputerQueries(queryClient, { includeComputers: true });
      queryClient.setQueryData<AccountStatusPayload | undefined>(accountKeys.status(), current =>
        current
          ? {
              ...current,
              signed_in: false,
              state: "signed_out",
              signed_out_reason: "signed_out",
              recent_sign_ins: [],
            }
          : current
      );
      void queryClient.invalidateQueries({ queryKey: accountKeys.all });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      if (gate && gate !== "off") {
        void router.navigate({ to: "/sign-in", replace: true });
      }
    },
    onError: err => {
      if (err instanceof AccountApiError && err.code === "confirm_email_mismatch") {
        setDeleteError("That email doesn't match this account.");
        return;
      }
      if (err instanceof AccountApiError && err.code === "account_deleting") {
        setDeleteError("Your account is being deleted. Try again in a few minutes.");
        return;
      }
      setDeleteError(err instanceof Error ? err.message : "Failed to delete account.");
    },
  });

  return {
    tokens,
    isUpdatingEmail: emailMutation.isPending,
    createTokenOpen,
    setCreateTokenOpen: (open: boolean) => {
      setCreateTokenOpen(open);
      if (!open) setCreateTokenError(null);
    },
    createTokenError,
    isCreatingToken: createTokenMutation.isPending,
    createdToken,
    setCreatedToken,
    revokingTokenId,
    isDismissingNotices: dismissNoticesMutation.isPending,
    deleteOpen,
    setDeleteOpen: (open: boolean) => {
      setDeleteOpen(open);
      if (open) {
        void fetchAccountStatus(undefined, { refresh: true })
          .then(fresh => {
            queryClient.setQueryData(accountKeys.status(), fresh);
          })
          .catch(() => undefined);
      } else {
        setDeleteError(null);
      }
    },
    deleteError,
    isDeletingAccount: deleteAccountMutation.isPending,
    handleToggleMarketingOptIn: (next: boolean) => emailMutation.mutate(next),
    handleCreateToken: (name: string) => createTokenMutation.mutate(name),
    handleRevokeToken: (token: AccountTokenFixture) => revokeTokenMutation.mutate(token),
    handleDismissSignInNotices: () => dismissNoticesMutation.mutate(),
    handleConfirmDeleteAccount: () => deleteAccountMutation.mutate(),
  };
}
