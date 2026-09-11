// Suite: loopback 403 classification
// Invariant: only an HTTP 403 carrying one of the two explicit loopback daemon codes classifies.
// Any other status, a missing code, or an unrelated code says nothing about loopback capability
// and must not drive the loopback-only state — the seam stays narrow.
// Owning layer: the gateway system's forbidden-envelope classifier.
import { describe, expect, it } from "vitest";

import { classifyGatewayLoopback } from "../gateway-loopback";

describe("classifyGatewayLoopback", () => {
  it("Should classify only the two loopback daemon codes on a 403", () => {
    // UT-005: shapes frozen in the spec's DX contract.
    expect(classifyGatewayLoopback(403, { code: "loopback_mutation_required" })).toBe("mutation");
    expect(classifyGatewayLoopback(403, { code: "loopback_api_required" })).toBe("api");
  });

  it("Should ignore a 403 without a code or with an unrelated code", () => {
    // UT-006: ordinary authorization refusals are not loopback signals.
    expect(classifyGatewayLoopback(403, { error: "forbidden" })).toBeUndefined();
    expect(classifyGatewayLoopback(403, { code: "workspace_home_forbidden" })).toBeUndefined();
    expect(classifyGatewayLoopback(403, { code: "" })).toBeUndefined();
    expect(classifyGatewayLoopback(403, null)).toBeUndefined();
  });

  it("Should ignore non-403 responses even when the codes appear", () => {
    expect(classifyGatewayLoopback(401, { code: "loopback_mutation_required" })).toBeUndefined();
    expect(classifyGatewayLoopback(500, { code: "loopback_api_required" })).toBeUndefined();
    expect(classifyGatewayLoopback(200, { code: "loopback_mutation_required" })).toBeUndefined();
  });
});
