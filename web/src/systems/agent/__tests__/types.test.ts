import { describe, expectTypeOf, it } from "vitest";

import type { AgentMCPServer, AgentPayload, AgentResponse, AgentsResponse } from "../types";

describe("agent contract types", () => {
  it("keeps required and optional agent fields aligned with the generated contract", () => {
    expectTypeOf<AgentPayload>().toMatchTypeOf<{
      name: string;
      provider: string;
      prompt: string;
      command?: string;
      model?: string;
      permissions?: string;
      tools?: string[];
    }>();

    expectTypeOf<AgentMCPServer>().toMatchTypeOf<{
      name: string;
      transport?: string;
      command?: string;
      url?: string;
      auth?: {
        client_id?: string;
        issuer_url?: string;
        registration?: string;
        scopes?: string[];
      } | null;
      args?: string[];
      env?: Record<string, string>;
    }>();

    type AgentMCPAuth = NonNullable<AgentMCPServer["auth"]>;
    expectTypeOf<AgentMCPAuth["registration"]>().toEqualTypeOf<string | undefined>();
    expectTypeOf<Extract<"client_secret_ref", keyof AgentMCPAuth>>().toEqualTypeOf<never>();

    expectTypeOf<AgentsResponse>().toMatchTypeOf<{ agents: AgentPayload[] }>();
    expectTypeOf<AgentResponse>().toMatchTypeOf<{ agent: AgentPayload }>();
  });
});
