import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { DirectoryBrowseQuery, FSBrowseResponse, OnboardingStatus } from "../types";

export class OnboardingApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "OnboardingApiError";
  }
}

export async function fetchOnboardingStatus(signal?: AbortSignal): Promise<OnboardingStatus> {
  const { data, error, response } = await apiClient.GET("/api/onboarding", { signal });
  if (apiRequestFailed(response, error)) {
    throw new OnboardingApiError(
      defaultApiErrorMessage("Failed to load onboarding status", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load onboarding status").onboarding;
}

export async function completeOnboarding(signal?: AbortSignal): Promise<OnboardingStatus> {
  const { data, error, response } = await apiClient.POST("/api/onboarding/complete", { signal });
  if (apiRequestFailed(response, error)) {
    throw new OnboardingApiError(
      defaultApiErrorMessage("Failed to complete onboarding", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to complete onboarding").onboarding;
}

export async function resetOnboarding(signal?: AbortSignal): Promise<OnboardingStatus> {
  const { data, error, response } = await apiClient.DELETE("/api/onboarding", { signal });
  if (apiRequestFailed(response, error)) {
    throw new OnboardingApiError(
      defaultApiErrorMessage("Failed to reset onboarding", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to reset onboarding").onboarding;
}

export async function browseDirectory(
  query: DirectoryBrowseQuery,
  signal?: AbortSignal
): Promise<FSBrowseResponse> {
  const { data, error, response } = await apiClient.GET("/api/fs/browse", {
    params: {
      query: {
        ...(query.path ? { path: query.path } : {}),
        ...(query.showHidden ? { show_hidden: true } : {}),
        ...(query.dirsOnly ? { dirs_only: true } : {}),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new OnboardingApiError(
      defaultApiErrorMessage("Failed to browse directory", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to browse directory");
}
