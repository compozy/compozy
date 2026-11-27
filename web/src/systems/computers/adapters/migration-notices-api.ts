import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

export async function fetchMigrationNotices(signal?: AbortSignal) {
  const { data, error, response } = await apiClient.GET("/api/migration-notices", { signal });
  const message = "Couldn't load upgrade notices.";
  if (apiRequestFailed(response, error)) throw new Error(message);
  return requireResponseData(data, response, message);
}

export async function dismissMigrationNotice(key: string) {
  const { error, response } = await apiClient.POST("/api/migration-notices/{key}/dismiss", {
    params: { path: { key } },
  });
  if (apiRequestFailed(response, error)) throw new Error("Couldn't dismiss the upgrade notice.");
}
