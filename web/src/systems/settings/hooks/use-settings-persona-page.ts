import { useQuery } from "@tanstack/react-query";
import { useState, type SetStateAction } from "react";

import { useProfileReadScope } from "@/systems/profiles";

import { SettingsApiError } from "../adapters/settings-api";
import { settingsPersonaFilterForProfile, settingsPersonaOptions } from "../lib/query-options";
import type { SettingsPersonaFilter, SettingsPersonaSection } from "../types";
import { useSettingsPage } from "./use-settings-page";
import { useUpdateSettingsPersona } from "./use-settings-mutations";

type PersonaConfig = SettingsPersonaSection["config"];

function profileFilterKey(filter: SettingsPersonaFilter): string {
  return `${filter.scope ?? ""}:${filter.workspace_id ?? ""}:${filter.profile ?? ""}`;
}

export function useSettingsPersonaPage() {
  const profile = useProfileReadScope();
  const page = useSettingsPage({ currentSlug: "defaults" });
  const filter = settingsPersonaFilterForProfile(profile.destination);
  const filterKey = profileFilterKey(filter);
  const query = useQuery(settingsPersonaOptions(filter));
  const envelope = query.data ?? null;
  const [draftState, setDraftState] = useState<{
    draft: PersonaConfig | null;
    filterKey: string;
  }>({ draft: null, filterKey });
  const draft =
    draftState.filterKey === filterKey
      ? (draftState.draft ?? envelope?.config ?? null)
      : (envelope?.config ?? null);
  const setDraft = (update: SetStateAction<PersonaConfig | null>) => {
    setDraftState(current => {
      const currentDraft =
        current.filterKey === filterKey
          ? (current.draft ?? envelope?.config ?? null)
          : (envelope?.config ?? null);
      return {
        draft: typeof update === "function" ? update(currentDraft) : update,
        filterKey,
      };
    });
  };
  const mutation = useUpdateSettingsPersona();
  const mutationHere =
    mutation.variables !== undefined && profileFilterKey(mutation.variables.filter) === filterKey;
  const pendingHere = mutation.isPending && mutationHere;
  const isDirty =
    envelope !== null && draft !== null
      ? JSON.stringify(envelope.config) !== JSON.stringify(draft)
      : false;

  return {
    envelope,
    draft,
    setDraft,
    profileName: profile.destination,
    isLoading: query.isLoading,
    error: query.error instanceof Error ? query.error : null,
    isDirty,
    isSaving: pendingHere,
    saveError:
      mutationHere && mutation.error instanceof SettingsApiError
        ? mutation.error.message
        : mutationHere && mutation.error instanceof Error
          ? mutation.error.message
          : null,
    warnings: mutationHere ? mutation.data?.warnings : undefined,
    restart: page.restart,
    handleReset: () => {
      if (envelope !== null) setDraft(envelope.config);
    },
    handleSave: () => {
      if (draft === null || pendingHere) return;
      mutation.mutate({ body: { config: draft }, filter });
    },
    handleRetry: () => {
      void query.refetch();
    },
  };
}
