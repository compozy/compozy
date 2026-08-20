import type { Action as ViewAction, Confirmation, RowAction } from "@compozy/extension-sdk";

import type { HandlerRegistry } from "./handler-registry.js";
import { childNodes } from "./host-types.js";
import type { HostNode } from "./host-types.js";
import { isRecord, optionalString, requiredString } from "./serialization-utils.js";

export function serializeActions(owner: HostNode, handlers: HandlerRegistry): RowAction[] {
  const panel = childNodes(owner, "view-action-panel")[0];
  if (!panel) return [];
  const result: RowAction[] = [];
  for (const child of childNodes(panel)) {
    if (child.type === "view-action") {
      result.push(serializeAction(child, handlers, undefined, result.length === 0));
      continue;
    }
    if (child.type !== "view-action-section") continue;
    const section = optionalString(child.props.title);
    for (const action of childNodes(child, "view-action")) {
      result.push(serializeAction(action, handlers, section, result.length === 0));
    }
  }
  return result;
}

function serializeAction(
  node: HostNode,
  handlers: HandlerRegistry,
  section: string | undefined,
  primary: boolean
): RowAction {
  const destructive = node.props.style === "destructive";
  if (destructive && !node.props.confirmation) {
    throw new Error("destructive actions require confirmation");
  }
  const handler = handlers.bind(node, "onAction", node.props.onAction);
  const action = isRecord(node.props.action) ? parseAction(node.props.action) : undefined;
  const submitForm = node.props.submitForm === true;
  const targets = [handler !== undefined, action !== undefined, submitForm].filter(Boolean).length;
  if (targets !== 1) {
    throw new Error("actions require exactly one handler, target, or form submit");
  }

  const result: RowAction = { title: requiredString(node.props.title, "action title") };
  const icon = optionalString(node.props.icon);
  const shortcut = optionalString(node.props.shortcut);
  if (icon) result.icon = icon;
  if (section) result.section = section;
  if (primary) result.primary = true;
  if (destructive) result.destructive = true;
  if (isRecord(node.props.confirmation)) {
    result.confirmation = parseConfirmation(node.props.confirmation);
  }
  if (shortcut) result.shortcut = shortcut;
  if (handler) result.handler = handler;
  if (action) result.action = action;
  if (submitForm) result.submit_form = true;
  return result;
}

function parseConfirmation(record: Record<string, unknown>): Confirmation {
  const result: Confirmation = {
    title: requiredString(record.title, "confirmation title"),
    confirm: requiredString(record.confirm, "confirmation confirm"),
  };
  const body = optionalString(record.body);
  if (body) result.body = body;
  return result;
}

function parseAction(record: Record<string, unknown>): ViewAction {
  const result: ViewAction = { kind: requiredString(record.kind, "action kind") };
  for (const field of ["op", "tool", "view", "app", "url"] as const) {
    const value = optionalString(record[field]);
    if (value) result[field] = value;
  }
  if (isRecord(record.args)) result.args = record.args as NonNullable<ViewAction["args"]>;
  return result;
}
