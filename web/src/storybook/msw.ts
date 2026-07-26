import { HttpResponse, http, type HttpHandler } from "msw";

import { handlers as agentHandlers } from "@/systems/agent/mocks";
import { handlers as automationHandlers } from "@/systems/automation/mocks";
import { handlers as bridgeHandlers } from "@/systems/bridges/mocks";
import { handlers as daemonHandlers } from "@/systems/status/mocks";
import { handlers as extensionHandlers } from "@/systems/extensions/mocks";
import { handlers as knowledgeHandlers } from "@/systems/knowledge/mocks";
import { handlers as loopsHandlers } from "@/systems/loops/mocks";
import { handlers as marketplaceHandlers } from "@/systems/marketplace/mocks";
import { handlers as modelCatalogHandlers } from "@/systems/model-catalog/mocks";
import { handlers as networkHandlers } from "@/systems/network/mocks";
import { handlers as onboardingHandlers } from "@/systems/onboarding/mocks";
import { handlers as osHandlers } from "@/systems/os/mocks";
import { handlers as runtimeHandlers } from "@/systems/runtime/mocks";
import { handlers as sessionHandlers } from "@/systems/session/mocks";
import { handlers as settingsHandlers } from "@/systems/settings/mocks";
import { handlers as skillHandlers } from "@/systems/skill/mocks";
import { handlers as schedulerHandlers } from "@/systems/scheduler/mocks";
import { handlers as tasksHandlers } from "@/systems/tasks/mocks";
import { handlers as toolApprovalsHandlers } from "@/systems/tool-approvals/mocks";
import { handlers as vaultHandlers } from "@/systems/vault/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";

export type StorybookHandlerGroupName =
  | "agent"
  | "automation"
  | "bridges"
  | "daemon"
  | "design-system"
  | "extensions"
  | "guard"
  | "knowledge"
  | "loops"
  | "marketplace"
  | "model-catalog"
  | "network"
  | "onboarding"
  | "os"
  | "runtime"
  | "session"
  | "settings"
  | "skill"
  | "scheduler"
  | "tasks"
  | "tool-approvals"
  | "vault"
  | "workspace";

export type StorybookHandlerGroups = Record<StorybookHandlerGroupName, HttpHandler[]>;
export type StorybookHandlerOverrides = Partial<StorybookHandlerGroups>;

export const storybookSystemHandlerGroups: StorybookHandlerGroups = {
  agent: agentHandlers,
  automation: automationHandlers,
  bridges: bridgeHandlers,
  daemon: daemonHandlers,
  "design-system": [],
  extensions: extensionHandlers,
  knowledge: knowledgeHandlers,
  loops: loopsHandlers,
  marketplace: marketplaceHandlers,
  "model-catalog": modelCatalogHandlers,
  network: networkHandlers,
  onboarding: onboardingHandlers,
  os: osHandlers,
  runtime: runtimeHandlers,
  session: sessionHandlers,
  settings: settingsHandlers,
  skill: skillHandlers,
  scheduler: schedulerHandlers,
  tasks: tasksHandlers,
  "tool-approvals": toolApprovalsHandlers,
  vault: vaultHandlers,
  workspace: workspaceHandlers,
  guard: [
    http.all("/api/*", () => {
      console.error(
        "[MSW] Error: intercepted a local API request without a matching request handler."
      );
      return HttpResponse.error();
    }),
  ],
};

export function flattenStorybookHandlerGroups(
  groups: StorybookHandlerGroups | StorybookHandlerOverrides
) {
  return Object.values(groups).flat();
}

export const storybookSystemHandlers = flattenStorybookHandlerGroups(storybookSystemHandlerGroups);

function handlerSignature(handler: HttpHandler) {
  const method = String(handler.info.method);
  const path = String(handler.info.path);

  return `${method} ${path}`;
}

export function composeStorybookHandlerGroup(
  groupName: StorybookHandlerGroupName,
  overrides: HttpHandler[]
) {
  const overrideSignatures = new Set(overrides.map(handlerSignature));
  const retained = storybookSystemHandlerGroups[groupName].filter(
    handler => !overrideSignatures.has(handlerSignature(handler))
  );

  // Concrete paths before param paths so an override of `/api/agents/{name}`
  // (OpenAPI MSW registers `:name`) cannot shadow retained `/api/agents/catalog`.
  const isConcrete = (handler: HttpHandler) => {
    const path = String(handler.info.path);
    return !/\{[^/]+\}|:[^/]+/.test(path);
  };
  const overrideConcrete = overrides.filter(isConcrete);
  const overrideParam = overrides.filter(handler => !isConcrete(handler));
  const retainedConcrete = retained.filter(isConcrete);
  const retainedParam = retained.filter(handler => !isConcrete(handler));

  return [...overrideConcrete, ...retainedConcrete, ...overrideParam, ...retainedParam];
}

export function composeStorybookHandlerOverrides(overrides: StorybookHandlerOverrides) {
  return Object.fromEntries(
    Object.entries(overrides).map(([groupName, handlers]) => [
      groupName,
      composeStorybookHandlerGroup(groupName as StorybookHandlerGroupName, handlers),
    ])
  ) as StorybookHandlerOverrides;
}

export function storybookMswParameters(overrides: StorybookHandlerOverrides) {
  return {
    msw: {
      handlers: composeStorybookHandlerOverrides(overrides),
    },
  } as const;
}
