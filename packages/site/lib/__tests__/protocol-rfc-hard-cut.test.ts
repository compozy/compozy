import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");

const activeProtocolDocs = [
  "docs/rfcs/003_agh-network-v0.md",
  "docs/rfcs/004_agh-network-v1.md",
  "docs/_memory/glossary.md",
] as const;

function listMDXDocs(dir: string): string[] {
  return readdirSync(dir).flatMap(entry => {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      return listMDXDocs(fullPath);
    }
    return stat.isFile() && fullPath.endsWith(".mdx") ? [relative(repoRoot, fullPath)] : [];
  });
}

const workspaceQualifiedProtocolDocs = [
  "docs/rfcs/003_agh-network-v0.md",
  ...listMDXDocs(resolve(siteRoot, "content/protocol")),
  ...listMDXDocs(resolve(siteRoot, "content/runtime/core/network")),
  "packages/site/content/runtime/guides/coordinate-agents-over-network.mdx",
  "docs/_memory/glossary.md",
];

const envelopeKinds = new Set(["greet", "whois", "say", "capability", "receipt", "trace"]);
const conversationKinds = new Set(["say", "capability", "receipt", "trace"]);
const discoveryKinds = new Set(["greet", "whois"]);

type JSONRecord = Record<string, unknown>;

type ExampleEnvelope = JSONRecord & {
  protocol: string;
  kind: string;
};

type JSONExample = {
  path: string;
  index: number;
  value: unknown;
};

function readRepoFile(path: string): string {
  return readFileSync(resolve(repoRoot, path), "utf8");
}

function isRecord(value: unknown): value is JSONRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isEnvelope(value: unknown): value is ExampleEnvelope {
  return (
    isRecord(value) &&
    typeof value.protocol === "string" &&
    typeof value.kind === "string" &&
    envelopeKinds.has(value.kind)
  );
}

function jsonExamples(path: string): JSONExample[] {
  const content = readRepoFile(path);
  return [...content.matchAll(/```json\n([\s\S]*?)```/g)].map((match, index) => {
    const body = match[1] ?? "";
    try {
      return { path, index, value: JSON.parse(body) as unknown };
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`${path} JSON block ${index + 1} is invalid: ${detail}`);
    }
  });
}

function activeEnvelopeExamples(
  paths: readonly string[]
): Array<JSONExample & { value: ExampleEnvelope }> {
  return paths.flatMap(path =>
    jsonExamples(path).flatMap(example =>
      isEnvelope(example.value) ? [{ ...example, value: example.value }] : []
    )
  );
}

function stringField(envelope: ExampleEnvelope, field: string): string | undefined {
  const value = envelope[field];
  return typeof value === "string" ? value : undefined;
}

function hasField(envelope: ExampleEnvelope, field: string): boolean {
  return Object.prototype.hasOwnProperty.call(envelope, field);
}

function location(example: JSONExample): string {
  return `${relative(repoRoot, resolve(repoRoot, example.path))}#json-${example.index + 1}`;
}

describe("protocol RFC hard cut", () => {
  it("keeps active protocol docs free of removed wire terms", () => {
    const removedPatterns = [
      /\binteraction_id\b/,
      /kind\s*:\s*"direct"/,
      /"kind"\s*:\s*"direct"/,
      /\bKindDirect\b/,
      /\bDirectBody\b/,
      /direct as (?:a )?message kind/i,
      /message kind[^.\n]*direct/i,
    ];

    const violations = activeProtocolDocs.flatMap(path => {
      const content = readRepoFile(path);
      return removedPatterns.flatMap(pattern =>
        [
          ...content.matchAll(
            new RegExp(
              pattern.source,
              pattern.flags.includes("g") ? pattern.flags : `${pattern.flags}g`
            )
          ),
        ].map(match => `${path}: ${match[0]}`)
      );
    });

    expect(violations).toEqual([]);
  });

  it("keeps active public protocol docs on workspace-qualified v0 network identity", () => {
    const legacyPatterns = [
      /agh-network\/v2/,
      /agh\.network\.v2/,
      /ProtocolV2/,
      /006_agh-network-v2/,
      /agh\.network\.v0\.<channel>/,
      /agh\.network\.v1\.<channel>/,
      /agh\.network\.v0\.builders\.(?:broadcast|peer)/,
      /agh\.network\.v1\.builders\.(?:broadcast|peer)/,
      /\/api\/network\/(?:channels|peers|send|work)/,
    ];

    const violations = workspaceQualifiedProtocolDocs.flatMap(path => {
      const content = readRepoFile(path);
      return legacyPatterns.flatMap(pattern =>
        [
          ...content.matchAll(
            new RegExp(
              pattern.source,
              pattern.flags.includes("g") ? pattern.flags : pattern.flags + "g"
            )
          ),
        ].map(match => path + ": " + match[0])
      );
    });

    expect(violations).toEqual([]);
  });

  it("keeps RFC 003 as current in-process v0 and RFC 004 as future v1 trust work", () => {
    const rfc003 = readRepoFile("docs/rfcs/003_agh-network-v0.md");
    expect(rfc003).toContain("**Status:** Current runtime contract");
    expect(rfc003).toContain("agh-network/v0");
    expect(rfc003).toContain("AGH Runtime in-process delivery");
    expect(rfc003).toContain("The durable commit is acceptance");
    expect(rfc003).not.toContain("RFC 006");

    const rfc004 = readRepoFile("docs/rfcs/004_agh-network-v1.md");
    expect(rfc004).toContain("**Status:** Future draft profile");
    expect(rfc004).toContain("future auth/proofs/trust-profile work");
    expect(rfc004).toContain("[RFC 003: AGH Network v0](003_agh-network-v0.md)");
    expect(rfc004).not.toContain("RFC 006");
  });

  it("keeps active RFCs free of the retired broker binding", () => {
    const retiredBindingPatterns = [
      /\bNATS\b/,
      /\bJetStream\b/,
      /agh\.network\.v[01]\.<workspace_id>/,
      /broker[- ](?:backed|managed|cluster|subject)/i,
    ];

    const violations = activeProtocolDocs.flatMap(path => {
      const content = readRepoFile(path);
      return retiredBindingPatterns.flatMap(pattern =>
        [
          ...content.matchAll(
            new RegExp(
              pattern.source,
              pattern.flags.includes("g") ? pattern.flags : `${pattern.flags}g`
            )
          ),
        ].map(match => `${path}: ${match[0]}`)
      );
    });

    expect(violations).toEqual([]);
  });

  it("requires active RFC envelope examples to carry workspace_id", () => {
    const envelopes = activeEnvelopeExamples([
      "docs/rfcs/003_agh-network-v0.md",
      "docs/rfcs/004_agh-network-v1.md",
    ]);

    const violations = envelopes.flatMap(example => {
      const workspaceID = stringField(example.value, "workspace_id");
      return workspaceID ? [] : [location(example) + ": missing workspace_id"];
    });

    expect(violations).toEqual([]);
  });

  it("requires conversation-bearing examples to use one explicit surface container", () => {
    const envelopes = activeEnvelopeExamples([
      "docs/rfcs/003_agh-network-v0.md",
      "docs/rfcs/004_agh-network-v1.md",
    ]);

    const violations = envelopes.flatMap(example => {
      const envelope = example.value;
      const surface = stringField(envelope, "surface");
      const threadID = stringField(envelope, "thread_id");
      const directID = stringField(envelope, "direct_id");
      const where = location(example);

      if (conversationKinds.has(envelope.kind)) {
        const issues: string[] = [];
        if (surface !== "thread" && surface !== "direct") {
          issues.push(`${where}: ${envelope.kind} missing surface`);
        }
        if (surface === "thread" && (!threadID || hasField(envelope, "direct_id"))) {
          issues.push(`${where}: thread surface must set only thread_id`);
        }
        if (surface === "direct" && (!directID || hasField(envelope, "thread_id"))) {
          issues.push(`${where}: direct surface must set only direct_id`);
        }
        return issues;
      }

      if (discoveryKinds.has(envelope.kind)) {
        const forbidden = ["surface", "thread_id", "direct_id", "work_id"].filter(field =>
          hasField(envelope, field)
        );
        return forbidden.map(field => `${where}: ${envelope.kind} must not set ${field}`);
      }

      return [];
    });

    expect(violations).toEqual([]);
  });

  it("uses work_id only on lifecycle-bearing examples", () => {
    const envelopes = activeEnvelopeExamples([
      "docs/rfcs/003_agh-network-v0.md",
      "docs/rfcs/004_agh-network-v1.md",
    ]);

    const violations = envelopes.flatMap(example => {
      const envelope = example.value;
      const workID = stringField(envelope, "work_id");
      const where = location(example);
      const issues: string[] = [];

      if (envelope.kind === "receipt" || envelope.kind === "trace") {
        if (!workID) {
          issues.push(`${where}: ${envelope.kind} must carry work_id`);
        }
      }

      if (workID) {
        if (!workID.startsWith("work_")) {
          issues.push(`${where}: work_id must use work_ prefix`);
        }
        if (!conversationKinds.has(envelope.kind)) {
          issues.push(`${where}: ${envelope.kind} must not carry work_id`);
        }
        if (!stringField(envelope, "surface")) {
          issues.push(`${where}: work_id requires a conversation surface`);
        }
      }

      if (discoveryKinds.has(envelope.kind) && hasField(envelope, "work_id")) {
        issues.push(`${where}: discovery message must not carry work_id`);
      }

      return issues;
    });

    expect(violations).toEqual([]);
  });

  it("documents RFC 004 signed fields and proves examples carry them when present", () => {
    const rfc004 = readRepoFile("docs/rfcs/004_agh-network-v1.md");
    for (const signedField of [
      "`surface` when present",
      "`thread_id` when present",
      "`direct_id` when present",
      "`work_id` when present",
    ]) {
      expect(rfc004).toContain(signedField);
    }

    const verifiedExamples = activeEnvelopeExamples(["docs/rfcs/004_agh-network-v1.md"]).filter(
      example => isRecord(example.value.proof)
    );
    expect(verifiedExamples.length).toBeGreaterThanOrEqual(2);

    const violations = verifiedExamples.flatMap(example => {
      const envelope = example.value;
      const where = location(example);
      const surface = stringField(envelope, "surface");
      const issues: string[] = [];

      if (surface === "thread" && !stringField(envelope, "thread_id")) {
        issues.push(`${where}: signed thread example missing thread_id`);
      }
      if (surface === "direct" && !stringField(envelope, "direct_id")) {
        issues.push(`${where}: signed direct example missing direct_id`);
      }
      if (envelope.kind === "trace" && !stringField(envelope, "work_id")) {
        issues.push(`${where}: signed trace example missing work_id`);
      }

      return issues;
    });

    expect(violations).toEqual([]);
    expect(verifiedExamples.some(example => stringField(example.value, "work_id"))).toBe(true);
  });
});
