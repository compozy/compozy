import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = resolve(siteRoot, "content/docs");

const providersDoc = resolve(docsRoot, "agents/providers.mdx");
const modelCatalogDoc = resolve(docsRoot, "agents/model-catalog.mdx");
const configTomlDoc = resolve(docsRoot, "configuration/config-toml.mdx");

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

  it("documents the daemon-owned refresh lifetime and serialization rules", () => {
    const source = read(modelCatalogDoc);
    expect(source).toContain("context.WithoutCancel");
    expect(source).toContain("serialized");
    expect(source).toContain("coalesce");
    expect(source).toContain("refresh_request_id");
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
