import { useQueryClient } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";

import { fileProblem } from "../lib/agent-frontmatter";
import { agentSoulOptions } from "../lib/query-options";
import { composeSoulFile, countSoulChanges, soulFromPayload } from "../lib/agent-soul-form";
import type { AgentPayload, AgentSoulPayload } from "../types";
import { buildAuthoredFileResourceKey } from "./use-agent-authored-file-editor";
import { usePutAgentSoul, useValidateAgentSoul } from "./use-agent-soul";
import { useEditorDocument } from "./use-editor-document";

const SAVED_COPY = "Saved. It takes effect from the next session.";

/** SOUL.md behind What it does: the form, the raw file, and one guarded write. */
export function useWhatItDoesEditor(
  agent: AgentPayload,
  workspace: string | null,
  payload: AgentSoulPayload
) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  const put = usePutAgentSoul();
  const validate = useValidateAgentSoul();
  const scope = { name: agent.name, cacheWorkspace: workspace, profile: destination };
  const selector = workspace ? { workspace_id: workspace } : {};

  return useEditorDocument({
    resourceKey: buildAuthoredFileResourceKey(workspace, agent.name, "soul", destination),
    snapshot: soulFromPayload(payload),
    compose: composeSoulFile,
    countChanges: countSoulChanges,
    savedCopy: SAVED_COPY,
    validate: async body => {
      const result = await validate.mutateAsync({ ...scope, params: { body, ...selector } });
      return result.valid
        ? undefined
        : (fileProblem(result.diagnostics) ?? "Fix the file to save.");
    },
    write: async (body, expected_digest) => {
      const result = await put.mutateAsync({
        ...scope,
        params: { body, expected_digest, ...selector },
      });
      return soulFromPayload(result.soul);
    },
    reread: async () =>
      soulFromPayload(
        await queryClient.fetchQuery({
          ...agentSoulOptions(agent.name, workspace, destination),
          staleTime: 0,
        })
      ),
  });
}
