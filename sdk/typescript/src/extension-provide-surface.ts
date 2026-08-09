import type { ExtensionDefinition } from "./types.js";
import type { ExtensionHandler } from "./extension-contract.js";
import { normalizeStringList } from "./extension-runtime.js";

export const registerProvideSurface = Symbol("registerProvideSurface");

export interface ProvideSurfaceRegistration {
  method: string;
  handler: ExtensionHandler;
}

interface ExtensionProvideSurfacesOptions {
  handlers: Map<string, ExtensionHandler>;
  validateMethod: (method: string) => void;
  bindMethod: (method: string) => void;
  unbindMethod: (method: string) => void;
}

export class ExtensionProvideSurfaces {
  private readonly owners = new Map<string, string>();

  public constructor(private readonly options: ExtensionProvideSurfacesOptions) {}

  public owner(method: string): string | undefined {
    return this.owners.get(method);
  }

  public register(
    definition: ExtensionDefinition,
    capability: string,
    registrations: readonly ProvideSurfaceRegistration[]
  ): void {
    const cleanCapability = capability.trim();
    if (!cleanCapability) {
      throw new Error("provide capability is required");
    }
    if (registrations.length === 0) {
      throw new Error(`${cleanCapability} requires at least one service method`);
    }

    const prepared = registrations.map(registration => ({
      method: registration.method.trim(),
      handler: registration.handler,
    }));
    this.validateRegistrations(cleanCapability, prepared);

    const previousCapabilities = definition.capabilities;
    definition.capabilities = {
      ...previousCapabilities,
      provides: normalizeStringList([...(previousCapabilities?.provides ?? []), cleanCapability]),
    };
    for (const registration of prepared) {
      this.options.handlers.set(registration.method, registration.handler);
      this.owners.set(registration.method, cleanCapability);
    }

    try {
      for (const registration of prepared) {
        this.options.bindMethod(registration.method);
      }
    } catch (error) {
      for (const registration of prepared) {
        this.options.handlers.delete(registration.method);
        this.owners.delete(registration.method);
        this.options.unbindMethod(registration.method);
      }
      if (previousCapabilities === undefined) {
        delete definition.capabilities;
      } else {
        definition.capabilities = previousCapabilities;
      }
      throw error;
    }
  }

  private validateRegistrations(
    capability: string,
    registrations: readonly ProvideSurfaceRegistration[]
  ): void {
    const seen = new Set<string>();
    for (const registration of registrations) {
      if (!registration.method) {
        throw new Error(`${capability} service method is required`);
      }
      if (registration.method === "initialize") {
        throw new Error("initialize is reserved by the SDK");
      }
      if (typeof registration.handler !== "function") {
        throw new Error(`${registration.method} handler is required`);
      }
      if (seen.has(registration.method)) {
        throw new Error(`${registration.method} is declared more than once`);
      }
      seen.add(registration.method);
      if (this.options.handlers.has(registration.method) || this.owners.has(registration.method)) {
        throw new Error(`${registration.method} is already registered`);
      }
      this.options.validateMethod(registration.method);
    }
  }
}
