import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import type { SkillExposeFailureResponse, SkillExposeRequest } from "../types";

import {
  skillActionFixture,
  skillContentFixtures,
  skillExposePartialFailureFixture,
  skillExposeSuccessFixture,
  skillFixtures,
  skillShadowsFixtures,
} from "./fixtures";

const skillByName = new Map(skillFixtures.map(skill => [skill.name, skill]));

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/skills", () => HttpResponse.json({ skills: skillFixtures })),
  compozyApiMock.get("/api/skills/{name}", ({ params }) => {
    const name = String(params.name);
    const skill = skillByName.get(name);

    if (!skill) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ skill });
  }),
  compozyApiMock.get("/api/skills/{name}/content", ({ params }) => {
    const name = String(params.name);
    const content = skillContentFixtures[name];

    if (!content) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ content });
  }),
  compozyApiMock.get("/api/skills/{name}/shadows", ({ params }) => {
    const name = String(params.name);
    const shadows = skillShadowsFixtures[name];

    if (!shadows) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json(shadows);
  }),
  compozyApiMock.post("/api/skills/{name}/enable", ({ params }) => {
    if (!skillByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Skill not found: ${String(params.name)}` },
        { status: 404 }
      );
    }

    return HttpResponse.json(skillActionFixture);
  }),
  compozyApiMock.post("/api/skills/{name}/disable", ({ params }) => {
    if (!skillByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Skill not found: ${String(params.name)}` },
        { status: 404 }
      );
    }

    return HttpResponse.json(skillActionFixture);
  }),
  // Exposing into `claude` hits an occupied path, which is the fixture's way of
  // exercising the one failure envelope both verbs share.
  compozyApiMock.post("/api/skills/{name}/expose", async ({ params, request }) => {
    const name = String(params.name);
    const body = (await request.json()) as SkillExposeRequest;
    const targets = body.targets;
    if (!skillByName.has(name) || targets.length === 0) {
      const message = !skillByName.has(name)
        ? `Skill not found: ${name}`
        : "At least one expose target is required";
      const failure: SkillExposeFailureResponse = {
        error: {
          code: !skillByName.has(name) ? "skill_not_found" : "expose_target_invalid",
          message,
        },
        name,
        results: targets.map(target => ({
          target,
          ok: false,
          error: {
            code: !skillByName.has(name) ? "skill_not_found" : "expose_target_invalid",
            message,
          },
        })),
        rolled_back: false,
      };
      return HttpResponse.json(failure, { status: 409 });
    }
    if (targets.includes("claude")) {
      const results = targets.map(target =>
        target === "claude"
          ? {
              target,
              ok: false,
              error: {
                code: "expose_name_conflict",
                occupied_by: `/Users/ana/.claude/skills/${name}`,
              },
            }
          : { target, ok: false, error: { code: "expose_name_conflict" } }
      );
      const failure: SkillExposeFailureResponse = {
        ...skillExposePartialFailureFixture,
        error: {
          code: "expose_failed",
          message: `${targets.length} ${targets.length === 1 ? "target" : "targets"} failed`,
        },
        name,
        results,
        rolled_back: false,
      };
      return HttpResponse.json(failure, { status: 409 });
    }
    return HttpResponse.json({
      ...skillExposeSuccessFixture,
      name,
      results: targets.map(target => ({
        target,
        ok: true,
        exposure: { target, path: `/Users/ana/.${target}/skills/${name}`, status: "healthy" },
      })),
      rolled_back: false,
    });
  }),
  compozyApiMock.post("/api/skills/{name}/unexpose", async ({ params, request }) => {
    const name = String(params.name);
    const body = (await request.json()) as SkillExposeRequest;
    if (!skillByName.has(name) || body.targets.length === 0) {
      const message = !skillByName.has(name)
        ? `Skill not found: ${name}`
        : "At least one unexpose target is required";
      const failure: SkillExposeFailureResponse = {
        error: {
          code: !skillByName.has(name) ? "skill_not_found" : "expose_target_invalid",
          message,
        },
        name,
        results: body.targets.map(target => ({
          target,
          ok: false,
          error: {
            code: !skillByName.has(name) ? "skill_not_found" : "expose_target_invalid",
            message,
          },
        })),
      };
      return HttpResponse.json(failure, { status: 409 });
    }
    if (body.targets.includes("claude")) {
      const results = body.targets.map(target =>
        target === "claude"
          ? {
              target,
              ok: false,
              error: {
                code: "expose_foreign_link",
                message: "The destination is not a CompozyOS-owned link",
              },
            }
          : { target, ok: true }
      );
      const failure: SkillExposeFailureResponse = {
        error: { code: "unexpose_failed", message: "One or more targets could not be removed" },
        name,
        results,
      };
      return HttpResponse.json(failure, { status: 409 });
    }
    return HttpResponse.json({
      name,
      results: body.targets.map(target => ({ target, ok: true })),
    });
  }),
];
