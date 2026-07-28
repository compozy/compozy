import { useInfiniteQuery } from "@tanstack/react-query";

import { sessionToolArtifactOptions } from "../lib/query-options";
import type { ToolArtifactPage } from "../types";

function decodeBase64(value: string): Uint8Array {
  const decoded = globalThis.atob(value);
  return Uint8Array.from(decoded, character => character.charCodeAt(0));
}

function decodeArtifactPages(pages: readonly ToolArtifactPage[]): {
  content: string;
  loadedBytes: number;
} {
  const chunks: Uint8Array[] = [];
  const offsets = new Set<number>();
  let loadedBytes = 0;
  for (const page of pages) {
    if (offsets.has(page.offset)) continue;
    offsets.add(page.offset);
    const chunk = decodeBase64(page.data_base64);
    chunks.push(chunk);
    loadedBytes += chunk.byteLength;
  }

  const combined = new Uint8Array(loadedBytes);
  let targetOffset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, targetOffset);
    targetOffset += chunk.byteLength;
  }
  return { content: new TextDecoder().decode(combined), loadedBytes };
}

export function useToolArtifact(workspaceId: string, artifactURI: string, opened: boolean) {
  const query = useInfiniteQuery(sessionToolArtifactOptions(workspaceId, artifactURI, opened));
  const decoded = decodeArtifactPages(query.data?.pages ?? []);
  const lastPage = query.data?.pages.at(-1);

  return {
    ...query,
    content: decoded.content,
    loadedBytes: decoded.loadedBytes,
    totalBytes: lastPage?.total_bytes ?? 0,
  };
}
