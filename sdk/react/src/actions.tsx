import type { Confirmation } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ReactElement, ReactNode } from "react";

import { useNavigation } from "./hooks/use-navigation.js";
import { queueViewEffect } from "./view-effects.js";

export interface ActionPanelProps {
  children?: ReactNode;
}

export interface ActionPanelSectionProps {
  title?: string;
  children?: ReactNode;
}

export interface ActionProps {
  title: string;
  icon?: string;
  shortcut?: string;
  style?: "default" | "destructive";
  confirmation?: Confirmation;
  onAction?: () => void | Promise<void>;
}

export interface ActionPushProps extends Omit<ActionProps, "onAction"> {
  target: ReactElement;
}

export interface ActionSubmitFormProps extends Omit<ActionProps, "onAction"> {}

export interface ActionOpenProps extends Omit<ActionProps, "onAction"> {
  url: string;
}

export interface ActionOpenAppProps extends Omit<ActionProps, "onAction"> {
  app: string;
}

export interface ActionCopyProps extends Omit<ActionProps, "onAction"> {
  content: string;
}

function ActionPanelRoot({ children }: ActionPanelProps): ReactElement {
  return createElement("view-action-panel", {}, children);
}

function ActionPanelSection({ children, ...props }: ActionPanelSectionProps): ReactElement {
  return createElement("view-action-section", props, children);
}

function ActionRoot(props: ActionProps): ReactElement {
  assertDestructiveConfirmation(props);
  return createElement("view-action", props);
}

function ActionPush({ target, ...props }: ActionPushProps): ReactElement {
  const { push } = useNavigation();
  return <ActionRoot {...props} onAction={() => push(target)} />;
}

function ActionSubmitForm(props: ActionSubmitFormProps): ReactElement {
  assertDestructiveConfirmation(props);
  return createElement("view-action", { ...props, submitForm: true });
}

function ActionOpen({ url, ...props }: ActionOpenProps): ReactElement {
  assertDestructiveConfirmation(props);
  return createElement("view-action", { ...props, action: { kind: "url", url } });
}

function ActionOpenApp({ app, ...props }: ActionOpenAppProps): ReactElement {
  assertDestructiveConfirmation(props);
  return createElement("view-action", { ...props, action: { kind: "navigate", app } });
}

function ActionCopyToClipboard({ content, ...props }: ActionCopyProps): ReactElement {
  return (
    <ActionRoot
      {...props}
      onAction={() => {
        queueViewEffect({ copy: { content } });
      }}
    />
  );
}

function assertDestructiveConfirmation(props: Pick<ActionProps, "style" | "confirmation">): void {
  if (props.style === "destructive" && !props.confirmation) {
    throw new Error("destructive actions require confirmation");
  }
}

export const ActionPanel = Object.assign(ActionPanelRoot, { Section: ActionPanelSection });

export const Action = Object.assign(ActionRoot, {
  Push: ActionPush,
  SubmitForm: ActionSubmitForm,
  Open: ActionOpen,
  OpenApp: ActionOpenApp,
  CopyToClipboard: ActionCopyToClipboard,
});
