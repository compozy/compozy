import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  archivePlanFixture,
  deletePlanFixture,
  profileFixtures,
  profileSelectionFixtures,
  renamePlanFixture,
} from "./fixtures";

function findProfile(name: string) {
  return profileFixtures.find(profile => profile.name === name);
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/profiles", () => HttpResponse.json(profileFixtures)),
  compozyApiMock.get("/api/profiles/selection", () => HttpResponse.json(profileSelectionFixtures)),
  compozyApiMock.put("/api/profiles/selection", async ({ request }) => {
    const body = (await request.json()) as { scope: string; profile: string };
    return HttpResponse.json(body);
  }),
  compozyApiMock.get("/api/profiles/ops", () => HttpResponse.json([])),
  compozyApiMock.get("/api/profiles/{name}", ({ params }) => {
    const profile = findProfile(String(params.name));
    if (!profile) {
      return HttpResponse.json(
        {
          error: {
            code: "profile_not_found",
            message: "That profile no longer exists.",
            action: "run compozy profile list",
          },
        },
        { status: 404 }
      );
    }
    return HttpResponse.json(profile);
  }),
  compozyApiMock.patch("/api/profiles/{name}", async ({ params, request }) => {
    const profile = findProfile(String(params.name));
    if (!profile) {
      return HttpResponse.json(
        {
          error: {
            code: "profile_not_found",
            message: "That profile no longer exists.",
            action: "run compozy profile list",
          },
        },
        { status: 404 }
      );
    }
    const patch = await request.json();
    return HttpResponse.json({ ...profile, ...patch, color: patch.color ?? profile.color });
  }),
  compozyApiMock.get("/api/profiles/{name}/rename-plan", () =>
    HttpResponse.json(renamePlanFixture)
  ),
  compozyApiMock.get("/api/profiles/{name}/archive-plan", () =>
    HttpResponse.json(archivePlanFixture)
  ),
  compozyApiMock.get("/api/profiles/{name}/delete-plan", () =>
    HttpResponse.json(deletePlanFixture)
  ),
];
