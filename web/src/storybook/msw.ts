import type { HttpHandler } from "msw";

import { handlers as appShellHandlers } from "@/systems/app-shell/mocks";
import { handlers as dashboardHandlers } from "@/systems/dashboard/mocks";
import { handlers as memoryHandlers } from "@/systems/memory/mocks";
import { handlers as reviewsHandlers } from "@/systems/reviews/mocks";
import { handlers as runsHandlers } from "@/systems/runs/mocks";
import { handlers as specHandlers } from "@/systems/spec/mocks";
import { handlers as workflowsHandlers } from "@/systems/workflows/mocks";

export type StorybookHandlerGroupName =
  | "appShell"
  | "dashboard"
  | "workflows"
  | "runs"
  | "reviews"
  | "spec"
  | "memory";

export type StorybookHandlerGroups = Record<StorybookHandlerGroupName, HttpHandler[]>;
export type StorybookHandlerOverrides = Partial<StorybookHandlerGroups>;

export const storybookSystemHandlerGroups: StorybookHandlerGroups = {
  appShell: appShellHandlers,
  dashboard: dashboardHandlers,
  workflows: workflowsHandlers,
  runs: runsHandlers,
  reviews: reviewsHandlers,
  spec: specHandlers,
  memory: memoryHandlers,
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

  return [
    ...overrides,
    ...storybookSystemHandlerGroups[groupName].filter(
      handler => !overrideSignatures.has(handlerSignature(handler))
    ),
  ];
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
