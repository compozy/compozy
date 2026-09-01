import { useMutation, useQueryClient } from "@tanstack/react-query";

import { updateSettingsAttention } from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import type {
  SettingsAttentionSection,
  SettingsMutationResult,
  SettingsUpdateAttentionFilter,
  SettingsUpdateAttentionRequest,
} from "../types";
import {
  invalidateSettingsApplyRecords,
  recordSettingsMutation,
} from "./settings-mutation-helpers";

type SettingsAttentionMutationVariables = {
  body: SettingsUpdateAttentionRequest;
  filter: SettingsUpdateAttentionFilter;
};

type StartedSettingsAttentionMutationVariables = SettingsAttentionMutationVariables & {
  request: Promise<SettingsMutationResult>;
};

function startSettingsAttentionMutation(
  variables: SettingsAttentionMutationVariables
): StartedSettingsAttentionMutationVariables {
  return {
    ...variables,
    request: updateSettingsAttention(variables.body, variables.filter),
  };
}

export function useUpdateSettingsAttention() {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: ({ request }: StartedSettingsAttentionMutationVariables) => request,
    onSuccess: (result, variables) => {
      recordSettingsMutation(result);
      queryClient.setQueryData<SettingsAttentionSection>(
        settingsKeys.attentionSection(variables.filter),
        previous =>
          previous
            ? {
                ...previous,
                config: {
                  ...previous.config,
                  ...variables.body.config,
                  muted_workspaces:
                    variables.body.config.muted_workspaces ?? previous.config.muted_workspaces,
                },
              }
            : previous
      );
    },
    onSettled: (_result, _error, variables) =>
      Promise.all([
        queryClient.invalidateQueries({
          queryKey: settingsKeys.attentionSection(variables.filter),
        }),
        invalidateSettingsApplyRecords(queryClient),
      ]),
  });

  return {
    ...mutation,
    mutate: (
      variables: SettingsAttentionMutationVariables,
      options?: Parameters<typeof mutation.mutate>[1]
    ) => mutation.mutate(startSettingsAttentionMutation(variables), options),
    mutateAsync: (
      variables: SettingsAttentionMutationVariables,
      options?: Parameters<typeof mutation.mutateAsync>[1]
    ) => mutation.mutateAsync(startSettingsAttentionMutation(variables), options),
  };
}
