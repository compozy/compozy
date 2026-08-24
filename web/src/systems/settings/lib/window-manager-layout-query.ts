import { queryOptions } from "@tanstack/react-query";

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
import { windowManagerSnapshotOptions } from "@/systems/os";

export interface WindowManagerLayoutReview {
  fingerprint: string;
  preview: WindowManagerLayoutPreview | null;
  validation: WindowManagerLayoutValidation;
}

export function windowManagerLayoutFingerprint(document: WindowManagerLayoutDocument): string {
  return JSON.stringify(windowManagerLayoutDocumentToWire(document));
}

export function windowManagerLayoutOptions(workspaceId: string, profile: string) {
  const normalized = workspaceId.trim();
  const snapshot = windowManagerSnapshotOptions(normalized, profile);
  return queryOptions({
    ...snapshot,
    select: windowManagerSnapshotToLayoutState,
  });
}

export function windowManagerLayoutProfilesOptions(workspaceId: string, profile: string) {
  const normalized = workspaceId.trim();
  return queryOptions({
    queryKey: settingsKeys.windowManagerLayoutProfiles(normalized, profile),
    queryFn: ({ signal }) => listWindowManagerLayoutProfiles(normalized, profile, signal),
    enabled: normalized !== "" && profile.trim() !== "",
    staleTime: 15_000,
  });
}

export function windowManagerLayoutReviewOptions(
  workspaceId: string,
  profile: string,
  revision: number,
  document: WindowManagerLayoutDocument
) {
  const normalized = workspaceId.trim();
  const fingerprint = windowManagerLayoutFingerprint(document);
  return queryOptions({
    queryKey: settingsKeys.windowManagerLayoutReview(normalized, profile, revision, fingerprint),
    queryFn: async ({ signal }): Promise<WindowManagerLayoutReview> => {
      const candidate = structuredClone(document);
      const validation = await validateWindowManagerLayout(normalized, profile, candidate, signal);
      if (!validation.valid) return { fingerprint, preview: null, validation };
      const preview = await previewWindowManagerLayout(
        normalized,
        profile,
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
