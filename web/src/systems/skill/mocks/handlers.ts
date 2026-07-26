import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  skillActionFixture,
  skillContentFixtures,
  skillFixtures,
  skillShadowsFixtures,
  skillMarketplaceInstallFixture,
  skillMarketplaceRemoveFixture,
  skillMarketplaceUpdateFixtures,
} from "./fixtures";

const skillByName = new Map(skillFixtures.map(skill => [skill.name, skill]));

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/skills", () => HttpResponse.json({ skills: skillFixtures })),
  aghApiMock.post("/api/skills/marketplace/install", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      slug?: string;
      version?: string;
    };
    if (!body.slug) {
      return HttpResponse.json({ error: "slug is required" }, { status: 400 });
    }
    if (body.slug !== skillMarketplaceInstallFixture.slug) {
      return HttpResponse.json(
        { error: `Marketplace skill not found: ${body.slug}` },
        { status: 404 }
      );
    }
    return HttpResponse.json({
      skill: {
        ...skillMarketplaceInstallFixture,
        slug: body.slug,
        version: body.version ?? skillMarketplaceInstallFixture.version ?? "0.0.0",
      },
    });
  }),
  aghApiMock.post("/api/skills/marketplace/update", async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      name?: string;
      all?: boolean;
      check_only?: boolean;
    };
    if (!body.name && !body.all) {
      return HttpResponse.json({ error: "name or all is required" }, { status: 400 });
    }
    return HttpResponse.json({ skills: skillMarketplaceUpdateFixtures });
  }),
  aghApiMock.delete("/api/skills/marketplace/{name}", ({ params }) => {
    const name = String(params.name);
    return HttpResponse.json({
      skill: { ...skillMarketplaceRemoveFixture, name },
    });
  }),
  aghApiMock.get("/api/skills/{name}", ({ params }) => {
    const name = String(params.name);
    const skill = skillByName.get(name);

    if (!skill) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ skill });
  }),
  aghApiMock.get("/api/skills/{name}/content", ({ params }) => {
    const name = String(params.name);
    const content = skillContentFixtures[name];

    if (!content) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json({ content });
  }),
  aghApiMock.get("/api/skills/{name}/shadows", ({ params }) => {
    const name = String(params.name);
    const shadows = skillShadowsFixtures[name];

    if (!shadows) {
      return HttpResponse.json({ error: `Skill not found: ${name}` }, { status: 404 });
    }

    return HttpResponse.json(shadows);
  }),
  aghApiMock.post("/api/skills/{name}/enable", ({ params }) => {
    if (!skillByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Skill not found: ${String(params.name)}` },
        { status: 404 }
      );
    }

    return HttpResponse.json(skillActionFixture);
  }),
  aghApiMock.post("/api/skills/{name}/disable", ({ params }) => {
    if (!skillByName.has(String(params.name))) {
      return HttpResponse.json(
        { error: `Skill not found: ${String(params.name)}` },
        { status: 404 }
      );
    }

    return HttpResponse.json(skillActionFixture);
  }),
];
