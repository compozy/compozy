import Reconciler from "react-reconciler";
import { DefaultEventPriority } from "react-reconciler/constants.js";
import type { ReactNode } from "react";

import type {
  HostChild,
  HostChildSet,
  HostContainer,
  HostNode,
  HostNodeType,
  HostProps,
  HostText,
} from "./host-types.js";

let currentUpdatePriority = DefaultEventPriority;
const rootHostContext = Object.freeze({});

const hostConfig = {
  rendererPackageName: "@compozy/extension-react",
  rendererVersion: "0.1.0",
  extraDevToolsConfig: null,
  supportsMutation: false,
  supportsPersistence: true,
  supportsHydration: false,
  isPrimaryRenderer: false,
  warnsIfNotActing: false,
  supportsMicrotasks: true,
  scheduleMicrotask: queueMicrotask,
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  noTimeout: -1,
  getRootHostContext: () => rootHostContext,
  getChildHostContext: () => rootHostContext,
  getPublicInstance: (instance: HostNode | HostText) => instance,
  prepareForCommit: () => null,
  resetAfterCommit: () => undefined,
  preparePortalMount: () => undefined,
  createInstance: (type: HostNodeType, props: HostProps): HostNode => ({
    type,
    props,
    children: [],
    handlerIDs: new Map(),
    hidden: false,
  }),
  createTextInstance: (value: string): HostText => ({ type: "text", value }),
  appendInitialChild: (parent: HostNode, child: HostChild) => {
    parent.children.push(child);
  },
  finalizeInitialChildren: () => false,
  shouldSetTextContent: () => false,
  cloneInstance: (
    instance: HostNode,
    type: HostNodeType,
    _oldProps: HostProps,
    newProps: HostProps,
    keepChildren: boolean
  ): HostNode => ({
    type,
    props: newProps,
    children: keepChildren ? instance.children : [],
    handlerIDs: new Map(instance.handlerIDs),
    hidden: instance.hidden,
  }),
  cloneHiddenInstance: (instance: HostNode, type: HostNodeType, props: HostProps): HostNode => ({
    type,
    props,
    children: instance.children,
    handlerIDs: new Map(instance.handlerIDs),
    hidden: true,
  }),
  cloneHiddenTextInstance: (instance: HostText): HostText => ({ ...instance }),
  createContainerChildSet: (): HostChildSet => [],
  appendChildToContainerChildSet: (children: HostChildSet, child: HostChild) => {
    children.push(child);
  },
  finalizeContainerChildren: () => undefined,
  replaceContainerChildren: (container: HostContainer, children: HostChildSet) => {
    container.children = children;
    container.onCommit();
  },
  setCurrentUpdatePriority: (priority: number) => {
    currentUpdatePriority = priority;
  },
  getCurrentUpdatePriority: () => currentUpdatePriority,
  resolveUpdatePriority: () => currentUpdatePriority || DefaultEventPriority,
  trackSchedulerEvent: () => undefined,
  resolveEventType: () => null,
  resolveEventTimeStamp: () => -1,
  shouldAttemptEagerTransition: () => false,
  detachDeletedInstance: () => undefined,
  maySuspendCommit: () => false,
  maySuspendCommitOnUpdate: () => false,
  maySuspendCommitInSyncRender: () => false,
  preloadInstance: () => true,
  startSuspendingCommit: () => undefined,
  suspendInstance: () => undefined,
  waitForCommitToBeReady: () => null,
  getSuspendedCommitReason: () => null,
  NotPendingTransition: null,
  HostTransitionContext: null,
  resetFormInstance: () => undefined,
  bindToConsole: (_method: string, args: unknown[]) => args,
  getInstanceFromNode: () => null,
  beforeActiveInstanceBlur: () => undefined,
  afterActiveInstanceBlur: () => undefined,
  prepareScopeUpdate: () => undefined,
  getInstanceFromScope: () => null,
  requestPostPaintCallback: (callback: (time: number) => void) => callback(Date.now()),
  supportsTestSelectors: false,
  findFiberRoot: () => null,
  getBoundingRect: () => ({ x: 0, y: 0, width: 0, height: 0 }),
  getTextContent: () => "",
  isHiddenSubtree: () => false,
  matchAccessibilityRole: () => false,
  setFocusIfFocusable: () => false,
  setupIntersectionObserver: () => ({ disconnect: () => undefined, observe: () => undefined }),
  clearContainer: (container: HostContainer) => {
    container.children = [];
    return false;
  },
};

export const viewReconciler = Reconciler(hostConfig as never);
export type ViewRoot = ReturnType<typeof viewReconciler.createContainer>;

interface React19Reconciler {
  updateContainerSync: (
    element: ReactNode,
    root: ViewRoot,
    parentComponent: null,
    callback: null
  ) => void;
  flushSyncWork: () => void;
  flushSyncFromReconciler: <T>(callback: () => T) => T;
}

const react19Reconciler = viewReconciler as unknown as React19Reconciler;

export function updateViewContainer(element: ReactNode, root: ViewRoot): void {
  react19Reconciler.updateContainerSync(element, root, null, null);
  react19Reconciler.flushSyncWork();
}

export function flushViewWork(): void {
  react19Reconciler.flushSyncFromReconciler(() => undefined);
}

export function runViewSync<T>(callback: () => T): T {
  return react19Reconciler.flushSyncFromReconciler(callback);
}
