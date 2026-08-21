import { registerViewProvider, type Extension, type ViewOpenRequest } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ComponentType } from "react";

import { ViewRenderer } from "./view-renderer.js";
import type { ViewRendererOptions } from "./view-renderer.js";

export type ReactViewComponent = ComponentType<Record<string, unknown>>;
export type ReactViewMap = Record<string, ReactViewComponent>;

export interface RegisterReactViewsOptions {
  diagnostics?: ViewRendererOptions["diagnostics"];
  now?: ViewRendererOptions["now"];
  scheduleFrame?: ViewRendererOptions["scheduleFrame"];
  starvationBudgetMS?: number;
}

interface ReactViewSession {
  renderer: ViewRenderer;
  controller: AbortController;
}

export function registerReactViews(
  extension: Extension,
  views: ReactViewMap,
  options: RegisterReactViewsOptions = {}
): Extension {
  const sessions = new Map<string, ReactViewSession>();
  return registerViewProvider(extension, {
    open: (context, request) => {
      const component = resolveView(views, request);
      const controller = new AbortController();
      const renderer = new ViewRenderer({
        viewSession: request.view_session,
        viewID: request.view,
        signal: controller.signal,
        publish: async frame => {
          await context.host.request("view/patch", frame);
        },
        ...(options.diagnostics ? { diagnostics: options.diagnostics } : {}),
        ...(options.now ? { now: options.now } : {}),
        ...(options.scheduleFrame ? { scheduleFrame: options.scheduleFrame } : {}),
        ...(options.starvationBudgetMS === undefined
          ? {}
          : { starvationBudgetMS: options.starvationBudgetMS }),
      });
      sessions.set(request.view_session, { renderer, controller });
      try {
        return renderer.open(createElement(component, request.args ?? {}));
      } catch (error) {
        sessions.delete(request.view_session);
        controller.abort(error);
        renderer.close();
        throw error;
      }
    },
    event: async (context, request) => {
      const session = sessions.get(request.view_session);
      if (!session) {
        throw new Error(`view session is not open: ${request.view_session}`);
      }
      return await session.renderer.event(
        request.handler,
        request.args ?? [],
        request.seq,
        request.generation,
        context.signal
      );
    },
    close: (_context, request) => {
      const session = sessions.get(request.view_session);
      sessions.delete(request.view_session);
      session?.controller.abort(new DOMException("View session closed", "AbortError"));
      session?.renderer.close();
    },
  });
}

function resolveView(views: ReactViewMap, request: ViewOpenRequest): ReactViewComponent {
  const shortID = request.view.split(".").at(-1) ?? request.view;
  const component = views[request.view] ?? views[shortID];
  if (!component) {
    throw new Error(`React view is not registered: ${request.view}`);
  }
  return component;
}
