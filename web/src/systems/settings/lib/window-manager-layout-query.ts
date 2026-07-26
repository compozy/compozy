import { queryOptions } from "@tanstack/react-query";

import { windowManagerSnapshotOptions } from "@/systems/os";

import {
  listWindowManagerLayoutProfiles,
  previewWindowManagerLayout,
  validateWindowManagerLayout,
} from "../adapters/window-manager-layouts-api";
import { settingsKeys } from "./query-keys";
import { windowManagerLayoutDocumentToWire } from "./window-manager-layout-schema";
import { windowManagerSnapshotToLayoutState } from "./window-manager-layout-projection";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutPreview,
  WindowManagerLayoutValidation,
} from "./window-manager-layout-types";

export interface WindowManagerLayoutReview {
  fingerprint: string;
  preview: WindowManagerLayoutPreview | null;
  validation: WindowManagerLayoutValidation;
}

export function windowManagerLayoutFingerprint(document: WindowManagerLayoutDocument): string {
  return JSON.stringify(windowManagerLayoutDocumentToWire(document));
}

export function windowManagerLayoutOptions(workspaceId: string) {
  const normalized = workspaceId.trim();
  const snapshot = windowManagerSnapshotOptions(normalized);
  return queryOptions({
    ...snapshot,
    select: windowManagerSnapshotToLayoutState,
  });
}

export function windowManagerLayoutProfilesOptions(workspaceId: string) {
  const normalized = workspaceId.trim();
  return queryOptions({
    queryKey: settingsKeys.windowManagerLayoutProfiles(normalized),
    queryFn: ({ signal }) => listWindowManagerLayoutProfiles(normalized, signal),
    enabled: normalized !== "",
    staleTime: 15_000,
  });
}

export function windowManagerLayoutReviewOptions(
  workspaceId: string,
  revision: number,
  document: WindowManagerLayoutDocument
) {
  const normalized = workspaceId.trim();
  const fingerprint = windowManagerLayoutFingerprint(document);
  return queryOptions({
    queryKey: settingsKeys.windowManagerLayoutReview(normalized, revision, fingerprint),
    queryFn: async ({ signal }): Promise<WindowManagerLayoutReview> => {
      const candidate = structuredClone(document);
      const validation = await validateWindowManagerLayout(normalized, candidate, signal);
      if (!validation.valid) return { fingerprint, preview: null, validation };
      const preview = await previewWindowManagerLayout(
        normalized,
        revision,
        candidate,
        undefined,
        signal
      );
      return { fingerprint, preview, validation };
    },
    enabled: false,
  });
}
