import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  archivePlanFixture,
  deletePlanFixture,
  profileFixtures,
  profileSelectionFixtures,
  renamePlanFixture,
} from "./fixtures";
import type {
  ProfileSelectionParams,
  ProfileSelectionResult,
  RenameProfilePlan,
  UpdateProfileParams,
} from "../types";

function findProfile(name: string) {
  return profileFixtures.find(profile => profile.name === name);
}

interface ProfileNotFoundResponse {
  error: { action: string; code: string; message: string };
}

function profileNotFound(): HttpResponse<ProfileNotFoundResponse> {
  return HttpResponse.json<ProfileNotFoundResponse>(
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

function renamePlanFor(profile: string): RenameProfilePlan {
  const replaceProfile = (value: string) => value.replaceAll("marketing", profile);
  return {
    ...renamePlanFixture,
    machine_folders: renamePlanFixture.machine_folders.map(replaceProfile),
    repo_candidates: renamePlanFixture.repo_candidates.map(candidate => ({
      ...candidate,
      path: replaceProfile(candidate.path),
    })),
    dormant_placements: renamePlanFixture.dormant_placements.map(placement => ({
      ...placement,
      profile,
    })),
  };
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/profiles", () => HttpResponse.json(profileFixtures)),
  compozyApiMock.get("/api/profiles/selection", () => HttpResponse.json(profileSelectionFixtures)),
  compozyApiMock.put("/api/profiles/selection", async ({ request }) => {
    const body = (await request.json()) as ProfileSelectionParams;
    return HttpResponse.json(body satisfies ProfileSelectionResult);
  }),
  compozyApiMock.get("/api/profiles/ops", () => HttpResponse.json([])),
  compozyApiMock.get("/api/profiles/{name}", ({ params }) => {
    const profile = findProfile(String(params.name));
    if (!profile) return profileNotFound();
    return HttpResponse.json(profile);
  }),
  compozyApiMock.patch("/api/profiles/{name}", async ({ params, request }) => {
    const profile = findProfile(String(params.name));
    if (!profile) return profileNotFound();
    const patch = (await request.json()) as UpdateProfileParams;
    return HttpResponse.json({ ...profile, ...patch, color: patch.color ?? profile.color });
  }),
  compozyApiMock.get("/api/profiles/{name}/rename-plan", ({ params, request }) => {
    const name = String(params.name);
    if (!findProfile(name)) return profileNotFound();
    const newName = new URL(request.url).searchParams.get("new_name")?.trim() ?? "";
    if (newName === "") {
      return HttpResponse.json(
        { error: { code: "invalid_profile", message: "New name is required.", action: "" } },
        { status: 400 }
      );
    }
    return HttpResponse.json(renamePlanFor(name));
  }),
  compozyApiMock.get("/api/profiles/{name}/archive-plan", ({ params }) =>
    findProfile(String(params.name)) ? HttpResponse.json(archivePlanFixture) : profileNotFound()
  ),
  compozyApiMock.get("/api/profiles/{name}/delete-plan", ({ params }) =>
    findProfile(String(params.name)) ? HttpResponse.json(deletePlanFixture) : profileNotFound()
  ),
];
