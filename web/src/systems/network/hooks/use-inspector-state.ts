import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

export type InspectorTab = "members" | "work";

interface InspectorChannelState {
  open: boolean;
  tab: InspectorTab;
}

interface NetworkInspectorContext {
  byWorkspace: Record<string, Record<string, InspectorChannelState>>;
}

export interface UseInspectorStateArgs {
  workspaceId: string | null | undefined;
  channel: string | null | undefined;
}

export const NETWORK_INSPECTOR_STORAGE_KEY = "compozy:network:inspector:v2";
const DEFAULT_STATE: InspectorChannelState = { open: false, tab: "members" };

function normalizedInspectorState(value: unknown): InspectorChannelState | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }

  const { open, tab } = value as { open?: unknown; tab?: unknown };
  if (typeof open !== "boolean" || (tab !== "members" && tab !== "work")) {
    return null;
  }
  return { open, tab };
}

function normalizedInspectorPreferences(
  value: unknown
): Record<string, Record<string, InspectorChannelState>> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }

  const byWorkspace: Record<string, Record<string, InspectorChannelState>> = {};
  for (const [workspaceId, channels] of Object.entries(value)) {
    if (typeof channels !== "object" || channels === null || Array.isArray(channels)) {
      continue;
    }

    const normalizedChannels: Record<string, InspectorChannelState> = {};
    for (const [channel, state] of Object.entries(channels)) {
      const normalized = normalizedInspectorState(state);
      if (normalized) {
        normalizedChannels[channel] = normalized;
      }
    }
    byWorkspace[workspaceId] = normalizedChannels;
  }
  return byWorkspace;
}

function mergeNetworkInspectorPreferences(
  persisted: Partial<NetworkInspectorContext>,
  current: NetworkInspectorContext
): NetworkInspectorContext {
  const persistedByWorkspace = normalizedInspectorPreferences(persisted.byWorkspace);
  const workspaceIds = new Set([
    ...Object.keys(current.byWorkspace),
    ...Object.keys(persistedByWorkspace),
  ]);
  const byWorkspace: Record<string, Record<string, InspectorChannelState>> = {};

  for (const workspaceId of workspaceIds) {
    byWorkspace[workspaceId] = {
      ...current.byWorkspace[workspaceId],
      ...persistedByWorkspace[workspaceId],
    };
  }

  return { byWorkspace };
}

function updatedInspectorState(
  context: NetworkInspectorContext,
  workspaceId: string,
  channel: string,
  update: (current: InspectorChannelState) => InspectorChannelState
): NetworkInspectorContext {
  const workspace = context.byWorkspace[workspaceId] ?? {};
  return {
    ...context,
    byWorkspace: {
      ...context.byWorkspace,
      [workspaceId]: {
        ...workspace,
        [channel]: update(workspace[channel] ?? DEFAULT_STATE),
      },
    },
  };
}

export const networkInspectorLogic = createStoreLogic({
  context: (): NetworkInspectorContext => ({ byWorkspace: {} }),
  on: {
    inspectorToggled: (context, event: { workspaceId: string; channel: string }) => {
      if (!event.workspaceId || !event.channel) {
        return;
      }
      return updatedInspectorState(context, event.workspaceId, event.channel, current => ({
        ...current,
        open: !current.open,
      }));
    },
    inspectorClosed: (context, event: { workspaceId: string; channel: string }) => {
      if (!event.workspaceId || !event.channel) {
        return;
      }
      return updatedInspectorState(context, event.workspaceId, event.channel, current => ({
        ...current,
        open: false,
      }));
    },
    inspectorOpened: (
      context,
      event: { workspaceId: string; channel: string; tab: InspectorTab }
    ) => {
      if (!event.workspaceId || !event.channel) {
        return;
      }
      return updatedInspectorState(context, event.workspaceId, event.channel, () => ({
        open: true,
        tab: event.tab,
      }));
    },
    inspectorTabSelected: (
      context,
      event: { workspaceId: string; channel: string; tab: InspectorTab }
    ) => {
      if (!event.workspaceId || !event.channel) {
        return;
      }
      return updatedInspectorState(context, event.workspaceId, event.channel, current => ({
        ...current,
        tab: event.tab,
      }));
    },
  },
});

export const networkInspectorStore = networkInspectorLogic.createStore().with(
  persist({
    name: NETWORK_INSPECTOR_STORAGE_KEY,
    merge: mergeNetworkInspectorPreferences,
  })
);

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === NETWORK_INSPECTOR_STORAGE_KEY) {
      void rehydrateStore(networkInspectorStore);
    }
  });
}

export interface UseInspectorStateResult {
  open: boolean;
  tab: InspectorTab;
  toggle: () => void;
  close: () => void;
  openWith: (tab: InspectorTab) => void;
  setTab: (tab: InspectorTab) => void;
}

/**
 * Per-workspace, per-channel inspector state. Preferences are local UI state;
 * Network members remain owned by the query layer.
 */
export function useInspectorState({
  workspaceId,
  channel,
}: UseInspectorStateArgs): UseInspectorStateResult {
  const open = useSelector(networkInspectorStore, snapshot =>
    workspaceId && channel
      ? (snapshot.context.byWorkspace[workspaceId]?.[channel]?.open ?? DEFAULT_STATE.open)
      : DEFAULT_STATE.open
  );
  const tab = useSelector(networkInspectorStore, snapshot =>
    workspaceId && channel
      ? (snapshot.context.byWorkspace[workspaceId]?.[channel]?.tab ?? DEFAULT_STATE.tab)
      : DEFAULT_STATE.tab
  );

  const toggle = () => {
    if (!workspaceId || !channel) {
      return;
    }
    networkInspectorStore.trigger.inspectorToggled({ workspaceId, channel });
  };

  const close = () => {
    if (!workspaceId || !channel) {
      return;
    }
    networkInspectorStore.trigger.inspectorClosed({ workspaceId, channel });
  };

  const openWith = (nextTab: InspectorTab) => {
    if (!workspaceId || !channel) {
      return;
    }
    networkInspectorStore.trigger.inspectorOpened({ channel, tab: nextTab, workspaceId });
  };

  const setTab = (nextTab: InspectorTab) => {
    if (!workspaceId || !channel) {
      return;
    }
    networkInspectorStore.trigger.inspectorTabSelected({ channel, tab: nextTab, workspaceId });
  };

  return { open, tab, toggle, close, openWith, setTab };
}
