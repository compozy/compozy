import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = resolve(siteRoot, "content/docs");

const providersDoc = resolve(docsRoot, "agents/providers.mdx");
const modelCatalogDoc = resolve(docsRoot, "agents/model-catalog.mdx");
const configTomlDoc = resolve(docsRoot, "configuration/config-toml.mdx");
const developExtensionsDoc = resolve(docsRoot, "extensions/develop.mdx");
const cliProviderModelsIndex = resolve(docsRoot, "cli/provider/models/index.mdx");
const cliProviderModelsList = resolve(docsRoot, "cli/provider/models/list.mdx");
const cliProviderModelsRefresh = resolve(docsRoot, "cli/provider/models/refresh.mdx");
const cliProviderModelsStatus = resolve(docsRoot, "cli/provider/models/status.mdx");

function read(path: string): string {
  return readFileSync(path, "utf8");
}

function collapseWhitespace(source: string): string {
  return source.replace(/\s+/g, " ").trim();
}

function nonHardCutMatches(source: string, pattern: RegExp): string[] {
  return source.split(/\r?\n/).flatMap(line => {
    if (
      line.match(/no longer|hard-cut|rejected with|deterministic hard-cut|are rejected|reject the/)
    ) {
      return [];
    }
    return line.match(pattern) ? [line] : [];
  });
}

describe("provider model catalog docs", () => {
  it("removes old provider model field claims from the providers doc", () => {
    const source = read(providersDoc);
    const offending = nonHardCutMatches(
      source,
      /\b(default_model|supported_models|supports_reasoning_effort)\b/
    );
    expect(offending).toEqual([]);
  });

  it("removes old provider model field claims from config.toml docs", () => {
    const source = read(configTomlDoc);
    const offending = nonHardCutMatches(
      source,
      /\b(default_model|supported_models|supports_reasoning_effort)\b/
    );
    expect(offending).toEqual([]);
  });

  it("documents the nested provider models block in the providers doc", () => {
    const source = read(providersDoc);
    expect(source).toContain("[providers.<id>.models]");
    expect(source).toContain("models.default");
    expect(source).toContain("models.curated");
    expect(source).toContain("models.discovery");
  });

  it("shows nested provider models examples only in the providers doc", () => {
    const source = read(providersDoc);
    expect(source).toContain("[providers.claude.models]");
    expect(source).toContain("[[providers.claude.models.curated]]");
    expect(source).toContain("[providers.openrouter.models]");
  });

  it("documents [model_catalog.sources.models_dev] in config.toml", () => {
    const source = read(configTomlDoc);
    expect(source).toContain("[model_catalog.sources.models_dev]");
    expect(source).toContain("https://models.dev/api.json");
    expect(source).toContain("ttl");
    expect(source).toContain("timeout");
  });

  it("documents provider models.discovery keys in config.toml", () => {
    const source = read(configTomlDoc);
    expect(source).toContain("models.discovery.enabled");
    expect(source).toContain("models.discovery.command");
    expect(source).toContain("models.discovery.endpoint");
    expect(source).toContain("models.discovery.timeout");
  });

  it("documents native model catalog endpoints", () => {
    const source = read(modelCatalogDoc);
    expect(source).toContain("/api/model-catalog/models");
    expect(source).toContain("/api/model-catalog/providers/{provider_id}/models");
    expect(source).toContain("/api/model-catalog/models/refresh");
    expect(source).toContain("/api/model-catalog/sources/status");
    expect(source).toContain("compozy__provider_models");
    expect(source).toContain("compozy__provider_models_list");
    expect(source).toContain("compozy__provider_models_refresh");
    expect(source).toContain("compozy__provider_models_status");
  });

  it("documents the OpenAI-compatible /api/openai/v1/models projection", () => {
    const source = read(modelCatalogDoc);
    expect(source).toContain("/api/openai/v1/models");
    expect(source).toContain("availability_state");
    expect(source).toContain("HTTP only");
  });

  it("documents the daemon-owned refresh lifetime and serialization rules", () => {
    const source = read(modelCatalogDoc);
    expect(source).toContain("context.WithoutCancel");
    expect(source).toContain("serialized");
    expect(source).toContain("coalesce");
    expect(source).toContain("refresh_request_id");
  });

  it("documents the model.source extension contract", () => {
    const source = read(developExtensionsDoc);
    expect(source).toContain("model.source");
    expect(source).toContain("models/list");
    expect(source).toContain("models/refresh");
    expect(source).toContain("models/status");
    expect(source).toContain("model.read");
    expect(source).toContain("model.write");
  });

  it("includes the regenerated provider models CLI reference", () => {
    const indexSource = read(cliProviderModelsIndex);
    expect(indexSource).toContain("compozy provider models");
    expect(indexSource).toContain("/docs/cli/provider/models/list");
    expect(indexSource).toContain("/docs/cli/provider/models/refresh");
    expect(indexSource).toContain("/docs/cli/provider/models/status");

    expect(read(cliProviderModelsList)).toContain("compozy provider models list");
    expect(read(cliProviderModelsRefresh)).toContain("compozy provider models refresh");
    expect(read(cliProviderModelsStatus)).toContain("compozy provider models status");
  });

  it("explains the compozy provider models namespace choice in the model catalog doc", () => {
    const source = read(modelCatalogDoc);
    expect(source).toContain("compozy provider models");
    expect(source).toContain("compozy models");
    expect(source).toContain("out of scope");
  });

  it("documents provider auth none and write-only local login constraints", () => {
    const providerSource = read(providersDoc);
    const providerText = collapseWhitespace(providerSource);
    const configSource = read(configTomlDoc);

    for (const source of [providerSource, configSource]) {
      expect(source).toContain("none_security");
      expect(source).toContain("No auth required");
      expect(source).toContain("local_transport");
      expect(source).toContain("external_identity");
      expect(source).toContain("public_readonly");
      expect(source).toContain("credential_slots`, `auth_status_command`, or `auth_login_command`");
      expect(source).toContain("providers.<id>.aliases");
      expect(source).toContain("Reference providers by canonical");
      expect(source).toContain("name only");
    }

    expect(providerText).toContain("executes the configured login command locally");
    expect(providerSource).toContain("write-only configuration input");
    expect(providerSource).toContain("safe login descriptor");
    expect(providerSource).not.toContain("--print-command");
    expect(providerSource).toContain("--no-tty");
    expect(providerSource).toContain("--timeout");
    expect(providerSource).toContain("never execute login commands");
  });
});
