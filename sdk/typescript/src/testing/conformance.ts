import { PUBLIC_PROVIDE_CONFORMANCE_FIXTURES } from "../generated/contracts.js";

export interface ProvideFixture {
  readonly provide: string;
  readonly requiredMethods: readonly string[];
}

export function publicProvideFixtures(): readonly ProvideFixture[] {
  return PUBLIC_PROVIDE_CONFORMANCE_FIXTURES.map(fixture => ({
    provide: fixture.provide,
    requiredMethods: [...fixture.required_methods],
  }));
}

export function validateProvideConformance(
  provide: string,
  implementedMethods: readonly string[]
): void {
  const fixture = publicProvideFixtures().find(candidate => candidate.provide === provide.trim());
  if (!fixture) {
    throw new Error(`provide ${JSON.stringify(provide.trim())} has no public conformance fixture`);
  }
  const implemented = new Set(implementedMethods);
  const missing = fixture.requiredMethods.filter(method => !implemented.has(method));
  if (missing.length > 0) {
    throw new Error(
      `provide ${JSON.stringify(fixture.provide)} is missing required methods: ${missing.join(", ")}`
    );
  }
}
