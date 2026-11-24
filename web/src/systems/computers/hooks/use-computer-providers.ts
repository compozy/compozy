import { use, useCallback, useMemo, useState } from "react";

import { toast } from "@compozy/ui";

import { remoteRejectionMessage } from "@/lib/remote-error-copy";
import { OsShellContext } from "@/systems/os";
import { useProfileReadScopeParams } from "@/systems/profiles";
import { useProviders } from "@/systems/providers";
import {
  createTerminal,
  holdPendingTerminalCommand,
  TerminalApiError,
} from "@/systems/terminal/parts";
import { itemBindingSearch, USER_SCOPE_SELECTOR } from "@/systems/workspace";

import { providerLoginCommand, toComputerProviderAuthView } from "../lib/provider-view";
import type {
  ComputerFixture,
  ComputerProviderAuthFixture,
} from "../mocks/remote-computers-types";

export interface ComputerProvidersController {
  providers: readonly ComputerProviderAuthFixture[];
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
  signingInProviderId: string | null;
  signInProvider: (provider: ComputerProviderAuthFixture, command?: string) => Promise<void>;
}

/**
 * Live provider sign-in readiness for a selected remote computer (`_uiux.md` U7,
 * `_user_stories.md` US-013) plus the `Sign in on <name>` remote terminal launcher.
 */
export function useComputerProviders(
  computer: ComputerFixture | null
): ComputerProvidersController {
  const shell = use(OsShellContext);
  const profileScope = useProfileReadScopeParams();
  const profile =
    "profile" in profileScope && profileScope.profile ? profileScope.profile : "default";
  const [signingInProviderId, setSigningInProviderId] = useState<string | null>(null);

  const canFetch = Boolean(computer && computer.status === "connected");
  const query = useProviders({
    computerId: computer?.id,
    enabled: canFetch,
  });

  const providers = useMemo<ComputerProviderAuthFixture[]>(() => {
    if (!query.data) return [];
    return query.data.providers.map(toComputerProviderAuthView);
  }, [query.data]);

  const signInProvider = useCallback(
    async (provider: ComputerProviderAuthFixture, explicitCommand?: string) => {
      if (!computer) return;
      const command = explicitCommand?.trim() || providerLoginCommand(provider);
      setSigningInProviderId(provider.id);
      try {
        const terminal = await createTerminal(
          USER_SCOPE_SELECTOR,
          { title: `${provider.name} sign-in` },
          { profile },
          null,
          undefined,
          computer.id
        );
        holdPendingTerminalCommand(computer.id, terminal.id, command);
        await shell?.coordinator.userOpen({
          app: "terminal",
          instanceKey: terminal.id,
          route: {
            pathname: `/terminal/${encodeURIComponent(terminal.id)}`,
            search: {
              ...itemBindingSearch(USER_SCOPE_SELECTOR),
              computer: computer.id,
            },
          },
        });
      } catch (error) {
        const message =
          error instanceof TerminalApiError
            ? remoteRejectionMessage(error.code, error.message, error.details)
            : error instanceof Error
              ? error.message
              : `Couldn't open a sign-in terminal on ${computer.name}.`;
        toast.error(message);
      } finally {
        setSigningInProviderId(null);
      }
    },
    [computer, profile, shell]
  );

  return {
    providers,
    isLoading: canFetch && query.isLoading,
    error: canFetch ? query.error : null,
    refetch: () => {
      void query.refetch();
    },
    signingInProviderId,
    signInProvider,
  };
}
