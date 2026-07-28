import { describe, expectTypeOf, it } from "vitest";

import type { HealthPayload, MemoryHealthPayload, StatusPayload } from "../types";

describe("daemon contract types", () => {
  it("derives health payloads from the generated status contract", () => {
    expectTypeOf<HealthPayload>().toMatchTypeOf<{
      status: string;
      uptime_seconds: number;
      active_sessions: number;
      active_agents: number;
      bridges: {
        total_instances: number;
        route_count: number;
        delivery_backlog: number;
        delivery_dropped_total: number;
        delivery_failures_total: number;
        auth_failures_total: number;
        status_counts: {
          disabled: number;
          starting: number;
          ready: number;
          degraded: number;
          auth_required: number;
          error: number;
        };
      };
      global_db_size_bytes: number;
      session_db_size_bytes: number;
      persistence: {
        status: string;
        global_db_size_bytes: number;
        session_db_size_bytes: number;
      };
      retention: {
        enabled: boolean;
        retention_days: number;
        sweep_interval_seconds: number;
        last_sweep_status: string;
        last_sweep_at?: string | null;
        last_cutoff_at?: string | null;
        last_sweep_error?: string;
        deleted_event_summaries: number;
        deleted_token_stats: number;
        deleted_permission_log_rows: number;
      };
      failures: {
        status: string;
        total: number;
        by_kind?: Record<string, number>;
        recent?: Array<{
          session_id: string;
          agent_name?: string;
          provider?: string;
          workspace_id?: string;
          state?: string;
          failure_kind: string;
          summary?: string;
          crash_bundle_path?: string;
          updated_at: string;
        }>;
      };
      agent_probes?: Array<{
        agent_name?: string;
        provider?: string;
        command?: string;
        executable?: string;
        status: string;
        error?: string;
        checked_at: string;
        duration_ms: number;
      }>;
      version: string;
    }>();

    expectTypeOf<MemoryHealthPayload>().toMatchTypeOf<{
      dream_enabled: boolean;
      global_files: number;
      workspace_files: number;
      last_consolidation: string | null;
    }>();

    expectTypeOf<StatusPayload>().toMatchTypeOf<{
      health: HealthPayload;
      memory: MemoryHealthPayload;
    }>();
  });
});
