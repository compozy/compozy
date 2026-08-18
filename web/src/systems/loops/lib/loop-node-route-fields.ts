import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import { idField, str } from "./loop-node-fields";
import type { FieldSpec, LoopRouteEntry, RoutesFieldSpec } from "./loop-node-schema-types";

export function readRoutes(raw: RawLoopNode): LoopRouteEntry[] {
  const routes = getAtPath(raw, ["routes"]);
  if (!Array.isArray(routes)) return [];
  return routes.map(entry => {
    const record =
      typeof entry === "object" && entry !== null ? (entry as Record<string, unknown>) : {};
    return { when: str(record.when), to: str(record.to) };
  });
}

export function routeFields(raw: RawLoopNode, targets: string[] = []): FieldSpec[] {
  const routesField: RoutesFieldSpec = {
    type: "routes",
    key: "routes",
    label: "Routes",
    path: ["routes"],
    defaultPath: ["default"],
    routes: readRoutes(raw),
    defaultRoute: str(getAtPath(raw, ["default"])),
    targets,
    hint: "Evaluated top to bottom; the first matching condition wins. Declaration order is the tiebreak.",
  };
  return [
    idField(),
    routesField,
    {
      type: "static",
      key: "on_eval_error",
      label: "on_eval_error",
      value: "fail",
      badge: "fail-closed",
      hint: "A routing predicate that cannot be evaluated stops the run rather than guessing a branch.",
    },
    {
      type: "hint",
      key: "hint",
      hint: "Sends the run down exactly one branch. Every route target must be forward of this node, and a default is required so a run always has somewhere to go.",
    },
  ];
}

export function routesEdit(routes: readonly LoopRouteEntry[]): NodeFieldEdit[] {
  return [{ path: ["routes"], value: routes.map(route => ({ when: route.when, to: route.to })) }];
}

export function routeDefaultEdit(target: string): NodeFieldEdit[] {
  return [{ path: ["default"], value: target === "" ? undefined : target }];
}
