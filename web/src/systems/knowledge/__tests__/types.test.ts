import { describe, expectTypeOf, it } from "vitest";

import type {
  KnowledgeFilter,
  KnowledgeSelector,
  MemoryAgentTier,
  MemoryDecision,
  MemoryDecisionsResponse,
  MemoryDeleteResponse,
  MemoryDreamTriggerResponse,
  MemoryEditRequest,
  MemoryEditResponse,
  MemoryHeader,
  MemoryReadResponse,
  MemoryScope,
  MemorySearchRequest,
  MemorySearchResponse,
  MemoryType,
  MemoryWriteRequest,
  MemoryWriteResponse,
} from "../types";

describe("knowledge contract types", () => {
  it("Should keep memory payloads aligned with the generated Memory v2 contract", () => {
    expectTypeOf<MemoryType>().toEqualTypeOf<"user" | "feedback" | "project" | "reference">();
    expectTypeOf<MemoryScope>().toEqualTypeOf<"global" | "workspace" | "agent" | undefined>();
    expectTypeOf<MemoryAgentTier>().toEqualTypeOf<"workspace" | "global" | undefined>();

    expectTypeOf<MemoryHeader>().toMatchTypeOf<{
      filename: string;
      mod_time: string;
      name: string;
      type: Exclude<MemoryType, undefined>;
      scope: "global" | "workspace" | "agent";
      recall_count: number;
      injection: boolean;
      system_managed: boolean;
      description?: string;
      agent_name?: string;
      agent_tier?: "workspace" | "global";
      staleness_banner?: string;
      superseded_by?: string;
      last_recalled_at?: string | null;
      workspace_id?: string;
    }>();

    expectTypeOf<MemoryReadResponse>().toMatchTypeOf<{
      memory: { content: string; summary: MemoryHeader };
    }>();

    expectTypeOf<MemoryWriteResponse>().toMatchTypeOf<{
      applied: boolean;
      decision: { id: string; candidate_hash: string; op: string };
    }>();
    expectTypeOf<MemoryEditResponse>().toMatchTypeOf<{
      applied: boolean;
      decision: { id: string; op: string };
    }>();
    expectTypeOf<MemoryDeleteResponse>().toMatchTypeOf<{
      applied: boolean;
      decision: { id: string; op: string };
    }>();

    expectTypeOf<MemoryDreamTriggerResponse>().toMatchTypeOf<{
      triggered: boolean;
      dream: { id: string; status: string };
      reason?: string;
    }>();

    expectTypeOf<MemorySearchRequest>().toMatchTypeOf<{
      query_text: string;
      scope?: "global" | "workspace" | "agent";
      workspace_id?: string;
      agent_name?: string;
      agent_tier?: "workspace" | "global";
      top_k?: number;
      include_system?: boolean;
    }>();

    expectTypeOf<MemorySearchResponse>().toMatchTypeOf<{
      results: { score: number; memory: { filename: string } }[];
    }>();

    expectTypeOf<MemoryDecisionsResponse>().toMatchTypeOf<{
      decisions: MemoryDecision[];
    }>();
    expectTypeOf<MemoryDecision>().toMatchTypeOf<{
      id: string;
      op: "noop" | "add" | "update" | "delete" | "reject";
      source: "rule" | "llm";
      candidate_hash: string;
      decided_at: string;
      confidence: number;
    }>();

    expectTypeOf<MemoryEditRequest>().toMatchTypeOf<{
      content: string;
    }>();

    expectTypeOf<MemoryWriteRequest>().toMatchTypeOf<{
      scope: "global" | "workspace" | "agent";
      type: MemoryType;
      name: string;
      content: string;
    }>();

    expectTypeOf<KnowledgeSelector>().toMatchTypeOf<{
      scope: "global" | "workspace" | "agent";
      workspaceId?: string;
      agentName?: string;
      agentTier?: "workspace" | "global";
    }>();

    expectTypeOf<KnowledgeFilter>().toMatchTypeOf<{
      scope: "global" | "workspace" | "agent";
      type?: MemoryType;
      search?: string;
    }>();
  });
});
