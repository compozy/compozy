// Suite: pairing artifact containment
// Invariant: a single-use pairing artifact never reaches a server-visible surface. It travels in
// the URL fragment, is stripped from the address bar before anything leaves the page, and reaches
// the daemon only in the redeem request body.
// Owning layer: the gateway pairing transfer helpers.
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { buildPairingLink, clearPairingFragment, readPairingFragment } from "../pairing-fragment";

const ARTIFACT = "cpz_gwp_2f8c1d5a9b";

function visit(url: string): void {
  window.history.replaceState(null, "", url);
}

describe("pairing fragment", () => {
  beforeEach(() => {
    visit("/");
  });

  afterEach(() => {
    visit("/");
  });

  it("Should keep the artifact out of the request line when building a scannable link", () => {
    const link = buildPairingLink("https://private.gateway.test", ARTIFACT);
    const parsed = new URL(link);

    expect(parsed.hash).toBe(`#pair=${ARTIFACT}`);
    expect(parsed.search).toBe("");
    expect(parsed.pathname).toBe("/");
    // Everything a server or proxy could log is the part before the fragment.
    expect(link.slice(0, link.indexOf("#"))).not.toContain(ARTIFACT);
  });

  it("Should not repeat a trailing slash from the advertised address", () => {
    expect(buildPairingLink("https://private.gateway.test/", ARTIFACT)).toBe(
      `https://private.gateway.test/#pair=${ARTIFACT}`
    );
  });

  it("Should read the artifact a scanned link delivered in the fragment", () => {
    visit(`/#pair=${ARTIFACT}`);
    expect(readPairingFragment()).toBe(ARTIFACT);
  });

  it("Should ignore a query parameter, which is a surface the artifact must never use", () => {
    visit(`/?pair=${ARTIFACT}`);
    expect(readPairingFragment()).toBe("");
  });

  it("Should strip the artifact from the address bar", () => {
    visit(`/#pair=${ARTIFACT}`);
    clearPairingFragment();

    expect(window.location.hash).toBe("");
    expect(window.location.href).not.toContain(ARTIFACT);
    expect(readPairingFragment()).toBe("");
  });

  it("Should preserve unrelated fragment state while removing the artifact", () => {
    visit(`/#tab=devices&pair=${ARTIFACT}`);
    clearPairingFragment();

    expect(window.location.hash).toBe("#tab=devices");
    expect(window.location.href).not.toContain(ARTIFACT);
  });

  it("Should leave a page without a pairing fragment untouched", () => {
    visit("/#tab=devices");
    clearPairingFragment();
    expect(window.location.hash).toBe("#tab=devices");
  });
});
