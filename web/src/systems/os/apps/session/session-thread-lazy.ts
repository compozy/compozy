import { lazy } from "react";

import { loadSessionThread } from "./session-window-module-loader";

export const SessionThread = lazy(() =>
  loadSessionThread().then(module => ({ default: module.SessionThread }))
);
