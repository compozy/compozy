import { createStoreLogic } from "@xstate/store";

import { emptyDraft, type MCPDraft } from "@/systems/settings";

interface HeaderCredential {
  id: string;
  key: string;
  value: string;
}

interface AgentMCPAddDialogState {
  mode: "pick" | "new";
  query: string;
  /** The person's checkbox changes only; checked state is derived from the agent + staging. */
  overrides: Record<string, boolean>;
  draft: MCPDraft;
  headers: HeaderCredential[];
}

/** One mounted dialog owns its draft; Query owns configured servers and Vault inventory. */
export const agentMCPAddDialogLogic = createStoreLogic({
  context: (): AgentMCPAddDialogState => ({
    mode: "pick",
    query: "",
    overrides: {},
    draft: emptyDraft(),
    headers: [],
  }),
  on: {
    newServerRequested: context => ({
      ...context,
      mode: "new" as const,
      draft: emptyDraft(),
      headers: [],
    }),
    pickerRequested: context => ({ ...context, mode: "pick" as const }),
    queryChanged: (context, event: { query: string }) => ({ ...context, query: event.query }),
    serverToggled: (context, event: { name: string; checked: boolean }) => ({
      ...context,
      overrides: { ...context.overrides, [event.name]: event.checked },
    }),
    draftChanged: (context, event: { draft: MCPDraft }) => ({ ...context, draft: event.draft }),
    headerAdded: (context, event: { id: string }) => ({
      ...context,
      headers: [
        ...context.headers,
        { id: event.id, key: context.headers.length === 0 ? "Authorization" : "", value: "" },
      ],
    }),
    headerChanged: (
      context,
      event: { id: string; patch: Partial<Pick<HeaderCredential, "key" | "value">> }
    ) => ({
      ...context,
      headers: context.headers.map(header =>
        header.id === event.id ? { ...header, ...event.patch } : header
      ),
    }),
    headerRemoved: (context, event: { id: string }) => ({
      ...context,
      headers: context.headers.filter(header => header.id !== event.id),
    }),
  },
});
