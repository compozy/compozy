export function formatDataPreview(data: unknown): string | null {
  if (data === undefined || data === null) {
    return null;
  }

  if (typeof data === "string") {
    return data;
  }

  try {
    return JSON.stringify(data) ?? String(data);
  } catch {
    return String(data);
  }
}
