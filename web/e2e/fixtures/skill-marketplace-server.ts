import { createServer, type Server, type ServerResponse } from "node:http";

import type { BrowserRuntimeSeed, BrowserSkillMarketplaceListingSeed } from "./runtime-seed";

const DEFAULT_HOST = "127.0.0.1";

export interface SkillMarketplaceTestServer {
  baseURL: string;
  server: Server;
}

export async function startSkillMarketplaceServer(
  seed: BrowserRuntimeSeed["skillMarketplace"] | undefined
): Promise<SkillMarketplaceTestServer | undefined> {
  if (seed === undefined || seed.listings.length === 0) {
    return undefined;
  }

  const listings = seed.listings.map(normalizeSkillMarketplaceListing);
  const server = createServer((request, response) => {
    const requestURL = new URL(request.url ?? "/", `http://${DEFAULT_HOST}`);
    if (request.method !== "GET") {
      respondWithError(response, 404, "not_found");
      return;
    }
    if (requestURL.pathname.startsWith("/api/v1/skills/")) {
      serveSkillMarketplaceDetail(requestURL, listings, response);
      return;
    }
    if (requestURL.pathname !== "/api/v1/search") {
      respondWithError(response, 404, "not_found");
      return;
    }
    if (requestURL.searchParams.get("type") !== "skill") {
      respondWithError(response, 400, "skill_type_required");
      return;
    }

    const query = requestURL.searchParams.get("q")?.trim().toLowerCase() ?? "";
    const limit = parseMarketplaceLimit(requestURL.searchParams.get("limit"), listings.length);
    const results = (
      query ? listings.filter(listing => marketplaceListingMatchesQuery(listing, query)) : listings
    )
      .slice(0, limit)
      .map(skillMarketplaceSearchPayload);

    response.statusCode = 200;
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ results }));
  });

  await new Promise<void>((resolve, reject) => {
    const handleError = (error: Error) => {
      cleanup();
      reject(error);
    };
    const handleListening = () => {
      cleanup();
      resolve();
    };
    const cleanup = () => {
      server.off("error", handleError);
      server.off("listening", handleListening);
    };

    server.once("error", handleError);
    server.once("listening", handleListening);
    server.listen(0, DEFAULT_HOST);
  });

  const address = server.address();
  if (address === null || typeof address === "string") {
    await closeSkillMarketplaceServer(server);
    throw new Error("failed to resolve browser skill marketplace server address");
  }

  return {
    baseURL: `http://${DEFAULT_HOST}:${address.port}`,
    server,
  };
}

function normalizeSkillMarketplaceListing(listing: BrowserSkillMarketplaceListingSeed) {
  return {
    author: listing.author ?? "",
    description: listing.description ?? "",
    downloads: listing.downloads ?? 0,
    name: listing.name,
    license: listing.license ?? "MIT",
    readme:
      listing.readme ??
      `---\nname: ${JSON.stringify(listing.name)}\ndescription: ${JSON.stringify(listing.description ?? listing.name)}\n---\n\n# ${listing.name}\n`,
    slug: listing.slug,
    source: listing.source ?? "clawhub",
    tags: listing.tags ?? [],
    type: "skill",
    version: listing.version ?? "",
  };
}

function serveSkillMarketplaceDetail(
  requestURL: URL,
  listings: ReturnType<typeof normalizeSkillMarketplaceListing>[],
  response: ServerResponse
): void {
  const encodedPath = requestURL.pathname.slice("/api/v1/skills/".length);
  if (encodedPath.endsWith("/download") || encodedPath.includes("/versions/")) {
    respondWithError(response, 404, "not_found");
    return;
  }

  const slug = decodeURIComponent(encodedPath);
  const listing = listings.find(candidate => candidate.slug === slug);
  if (listing === undefined) {
    respondWithError(response, 404, "not_found");
    return;
  }

  response.statusCode = 200;
  response.setHeader("content-type", "application/json");
  response.end(
    JSON.stringify({
      author: listing.author,
      description: listing.description,
      downloads: listing.downloads,
      license: listing.license,
      name: listing.name,
      readme: listing.readme,
      slug: listing.slug,
      tags: listing.tags,
      version: listing.version,
    })
  );
}

function marketplaceListingMatchesQuery(
  listing: ReturnType<typeof normalizeSkillMarketplaceListing>,
  query: string
): boolean {
  return [listing.name, listing.slug, listing.author, listing.description]
    .join(" ")
    .toLowerCase()
    .includes(query);
}

function skillMarketplaceSearchPayload(
  listing: ReturnType<typeof normalizeSkillMarketplaceListing>
) {
  return {
    author: listing.author,
    description: listing.description,
    downloads: listing.downloads,
    name: listing.name,
    slug: listing.slug,
    source: listing.source,
    type: listing.type,
    version: listing.version,
  };
}

function parseMarketplaceLimit(rawValue: string | null, fallback: number): number {
  const parsed = Number.parseInt(rawValue ?? "", 10);
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed;
  }
  return fallback;
}

function respondWithError(response: ServerResponse, statusCode: number, error: string): void {
  response.statusCode = statusCode;
  response.setHeader("content-type", "application/json");
  response.end(JSON.stringify({ error }));
}

export async function closeSkillMarketplaceServer(server: Server | undefined): Promise<void> {
  if (server === undefined || !server.listening) {
    return;
  }

  await new Promise<void>((resolve, reject) => {
    server.close(error => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
