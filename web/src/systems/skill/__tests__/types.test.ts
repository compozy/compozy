import { describe, expectTypeOf, it } from "vitest";

import type {
  ProvenancePayload,
  SkillActionResponse,
  SkillContentResponse,
  SkillMarketplaceInstallPayload,
  SkillMarketplaceInstallRequest,
  SkillMarketplaceRemovePayload,
  SkillMarketplaceUpdatePayload,
  SkillMarketplaceUpdateRequest,
  SkillPayload,
  SkillResponse,
  SkillsResponse,
} from "../types";

describe("skill contract types", () => {
  it("keeps skill payload fields aligned with the generated OpenAPI contract", () => {
    expectTypeOf<SkillPayload>().toMatchTypeOf<{
      name: string;
      description: string;
      source: string;
      enabled: boolean;
      dir: string;
      version?: string;
      metadata?: Record<string, unknown>;
      provenance?: ProvenancePayload | null;
    }>();

    expectTypeOf<ProvenancePayload>().toMatchTypeOf<{
      slug?: string;
      registry?: string;
      version?: string;
      installed_at?: string | null;
      installed_from_bundle?: string;
      installed_from_extension?: string;
      precedence_tier: string;
      shadowed_by?: {
        detected_at: string;
        path: string;
        resolved_to_winner: boolean;
        tier: string;
      }[];
    }>();

    expectTypeOf<SkillsResponse>().toMatchTypeOf<{ skills: SkillPayload[] }>();
    expectTypeOf<SkillResponse>().toMatchTypeOf<{ skill: SkillPayload }>();
    expectTypeOf<SkillContentResponse>().toMatchTypeOf<{ content: string }>();
    expectTypeOf<SkillActionResponse>().toEqualTypeOf<{ ok: boolean }>();
  });

  it("keeps marketplace payloads aligned with the generated OpenAPI contract", () => {
    expectTypeOf<SkillMarketplaceInstallPayload>().toMatchTypeOf<{
      name: string;
      slug: string;
      status: string;
      hash: string;
      path: string;
      registry: string;
    }>();

    expectTypeOf<SkillMarketplaceUpdatePayload>().toMatchTypeOf<{
      name: string;
      slug: string;
      status: string;
      path: string;
    }>();

    expectTypeOf<SkillMarketplaceRemovePayload>().toMatchTypeOf<{
      name: string;
      slug: string;
      status: string;
      path: string;
    }>();

    expectTypeOf<SkillMarketplaceInstallRequest>().toMatchTypeOf<{
      slug: string;
      version?: string;
    }>();

    expectTypeOf<SkillMarketplaceUpdateRequest>().toMatchTypeOf<{
      name?: string;
      all?: boolean;
      check_only?: boolean;
    }>();
  });
});
