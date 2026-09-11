/**
 * What the latched listener tier can execute, mirroring the backend SurfaceSet
 * route matrices (`internal/api/httpapi/surface_routes.go`).
 *
 * The backend remains the sole enforcement authority (BR-6): this map only
 * keeps the UI from offering affordances the tier's route matrix never
 * registers. It keys off the tier alone — no per-route discovery — so a web
 * unit test per flag plus the Go route-matrix tests own the drift risk.
 *
 * Both remote tiers (`private`, `public`) resolve to the same capability set:
 * their operator matrices differ only in gateway-management scope, which the
 * existing tier-aware gateway settings page already owns.
 */
import type { GatewayListenerTier } from "@/lib/gateway-access-signal";

export interface GatewayCapabilities {
  /**
   * Terminal routes and local-only task lifecycle (task/run mutations,
   * scheduler, task-review verdicts). The backend registers these only when
   * `includeLocalOnlyTaskLifecycle` is true, i.e. the local surface set
   * (`surface_routes.go:71-73`, `routes.go:25/85`, `routes.go:167-213`);
   * remote tiers register the read subset.
   */
  readonly localTaskLifecycle: boolean;
  /**
   * Settings/extension writes and other privileged mutations. These sit
   * behind `privilegedMutationGuard` → `loopbackMutationGuard`, which 403s
   * with `loopback_mutation_required` unless the daemon is loopback-bound
   * (`routes.go` privileged registrations; `handlers.go:311-317`,
   * `middleware.go:379-401`). Gateway listeners are non-loopback by
   * construction, so no remote tier can execute them.
   */
  readonly privilegedMutations: boolean;
  /**
   * Notification-preset enablement, extension enablement, and profile-state
   * writes. On non-local tiers the backend swaps these handlers for
   * `ProfileRemoteWriteForbidden` (403 "profile management is available only
   * on the local Compozy surface") — `routes.go:58-72`, `routes.go:368-395`,
   * `profile_routes.go:9-40`.
   */
  readonly profileEnablementWrites: boolean;
  /**
   * Agent kernel routes (`/api/agent-kernel/*`). Registered only on the local
   * surface set (`routes.go:26`); the private and public tier registrations
   * (`surface_routes.go:27-59`) never include them.
   */
  readonly agentKernel: boolean;
  /**
   * Full resource routes (`registerResourceRoutes`, local set only,
   * `routes.go:27`). Remote tiers register `registerRemoteResourceReadRoutes`
   * instead — a read subset (`surface_routes.go:34,56`).
   */
  readonly fullResourceRoutes: boolean;
}

/** The local surface set registers every operator route group. */
const LOCAL_CAPABILITIES: GatewayCapabilities = {
  localTaskLifecycle: true,
  privilegedMutations: true,
  profileEnablementWrites: true,
  agentKernel: true,
  fullResourceRoutes: true,
};

/**
 * Both remote tiers: read/monitor surface only. Every loopback- or
 * local-only group above is absent from their route matrices.
 */
const REMOTE_CAPABILITIES: GatewayCapabilities = {
  localTaskLifecycle: false,
  privilegedMutations: false,
  profileEnablementWrites: false,
  agentKernel: false,
  fullResourceRoutes: false,
};

/**
 * Capability set for a latched tier. `undefined` (tier not yet latched from
 * `/api/status`) resolves to the all-false set — loopback-only affordances
 * stay hidden until the tier proves `local` (BR-2, no flash-then-hide).
 */
export function capabilitiesForTier(tier: GatewayListenerTier | undefined): GatewayCapabilities {
  return tier === "local" ? LOCAL_CAPABILITIES : REMOTE_CAPABILITIES;
}
