import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse as parseYaml } from "yaml";
import { z } from "zod";
import { siteConfig } from "./site-config";

/**
 * What ships inside the binary, read from the same manifests the daemon reads.
 *
 * `extensions/spec-cycle` is enrolled by `speccycle.EnsureManagedInstall` at daemon boot with
 * `extensionpkg.SourceBundled` (`internal/daemon/boot_automation_bundles.go`). It therefore has no
 * catalog entry, no artifact URL, and no install command — publishing one would both misstate how it
 * arrives and collide with that managed install. The same is true of the skills embedded under
 * `skills/`: they are compiled in, not fetched from a registry.
 *
 * Counts are derived, never written down twice: the inventory is the directory listing plus the
 * manifest's own tool map.
 */

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const specCycleRoot = resolve(repoRoot, "extensions", "spec-cycle");
const bundledSkillsRoot = resolve(repoRoot, "skills");

export const BUNDLED_SPEC_CYCLE_PATH = "/marketplace/bundled/spec-cycle";

const toolSchema = z.object({
  display_title: z.string().min(1),
  description: z.string().min(1),
  risk: z.enum(["read", "mutating", "destructive"]),
  read_only: z.boolean(),
  concurrency_safe: z.boolean(),
  visibility: z.enum(["model", "operator", "hidden"]),
});

const resourcePathSchema = z.object({
  path: z.string().min(1),
  profile: z.string().min(1).optional(),
});

const resourcePathsSchema = z
  .array(resourcePathSchema)
  .transform(resources => resources.map(resource => resource.path));

const specCycleManifestSchema = z.object({
  extension: z.object({
    name: z.string().min(1),
    version: z.string().min(1),
    description: z.string().min(1),
    min_compozy_version: z.string().min(1),
  }),
  capabilities: z.object({ provides: z.array(z.string().min(1)).min(1) }),
  resources: z.object({
    skills: resourcePathsSchema,
    loops: resourcePathsSchema,
    agents: resourcePathsSchema,
    tools: z.record(z.string(), toolSchema),
  }),
});

const loopMetaSchema = z.object({
  meta: z.object({
    name: z.string().min(1),
    description: z.string().min(1),
    catalog: z
      .object({
        use_when: z.string().min(1).optional(),
        category: z.string().min(1).optional(),
      })
      .optional(),
  }),
});

const skillFrontmatterSchema = z.object({
  name: z.string().min(1),
  description: z.string().min(1),
});

export interface BundledTool {
  name: string;
  title: string;
  description: string;
  risk: z.infer<typeof toolSchema>["risk"];
  readOnly: boolean;
  concurrencySafe: boolean;
  visibility: z.infer<typeof toolSchema>["visibility"];
}

export interface BundledLoop {
  name: string;
  description: string;
  useWhen?: string;
  category?: string;
}

export interface BundledExtension {
  name: string;
  displayName: string;
  version: string;
  description: string;
  minCompozyVersion: string;
  provides: string[];
  loops: BundledLoop[];
  skills: string[];
  agents: string[];
  tools: BundledTool[];
  repositoryUrl: string;
  /** The only command that applies: it is already installed, so you can only inspect it. */
  statusCommand: string;
}

export interface BundledSkill {
  name: string;
  description: string;
  repositoryUrl: string;
}

interface ManifestDirectory {
  parent: string;
  name: string;
}

function listDirectories(root: string, parents: string[]): ManifestDirectory[] {
  const directories: ManifestDirectory[] = [];
  for (const parent of parents) {
    for (const entry of readdirSync(resolve(root, parent), { withFileTypes: true })) {
      if (entry.isDirectory()) {
        directories.push({ parent, name: entry.name });
      }
    }
  }
  return directories.sort(
    (left, right) => left.parent.localeCompare(right.parent) || left.name.localeCompare(right.name)
  );
}

function readLoop(parent: string, loopName: string): BundledLoop {
  const raw = readFileSync(resolve(specCycleRoot, parent, loopName, "loop.yaml"), "utf8");
  const { meta } = loopMetaSchema.parse(parseYaml(raw));
  return {
    name: meta.name,
    description: meta.description,
    useWhen: meta.catalog?.use_when,
    category: meta.catalog?.category,
  };
}

function readSpecCycle(): BundledExtension {
  const manifest = specCycleManifestSchema.parse(
    JSON.parse(readFileSync(resolve(specCycleRoot, "extension.json"), "utf8"))
  );
  const loopDirectories = listDirectories(specCycleRoot, manifest.resources.loops);
  return {
    name: manifest.extension.name,
    displayName: "Spec Cycle",
    version: manifest.extension.version,
    description: manifest.extension.description,
    minCompozyVersion: manifest.extension.min_compozy_version,
    provides: manifest.capabilities.provides,
    loops: loopDirectories.map(({ parent, name }) => readLoop(parent, name)),
    skills: listDirectories(specCycleRoot, manifest.resources.skills).map(({ name }) => name),
    agents: listDirectories(specCycleRoot, manifest.resources.agents).map(({ name }) => name),
    tools: Object.entries(manifest.resources.tools)
      .map(([name, tool]) => ({
        name,
        title: tool.display_title,
        description: tool.description,
        risk: tool.risk,
        readOnly: tool.read_only,
        concurrencySafe: tool.concurrency_safe,
        visibility: tool.visibility,
      }))
      .sort((left, right) => left.name.localeCompare(right.name)),
    repositoryUrl: `${siteConfig.repoUrl}/tree/${siteConfig.repoBranch}/extensions/spec-cycle`,
    statusCommand: `compozy extension status ${manifest.extension.name}`,
  };
}

export function parseBundledSkillFrontmatter(
  raw: string
): Pick<BundledSkill, "name" | "description"> {
  const match = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(raw);
  if (!match?.[1]) {
    throw new Error("SKILL.md must begin with YAML frontmatter");
  }
  return skillFrontmatterSchema.parse(parseYaml(match[1]));
}

/** `skills/embed.go` compiles every directory here into the binary. */
function readBundledSkills(): BundledSkill[] {
  const skills: BundledSkill[] = [];
  for (const entry of readdirSync(bundledSkillsRoot, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const raw = readFileSync(resolve(bundledSkillsRoot, entry.name, "SKILL.md"), "utf8");
    const { name, description } = parseBundledSkillFrontmatter(raw);
    skills.push({
      name,
      description,
      repositoryUrl: `${siteConfig.repoUrl}/tree/${siteConfig.repoBranch}/skills/${entry.name}`,
    });
  }
  return skills.sort((left, right) => left.name.localeCompare(right.name));
}

export const specCycleExtension: BundledExtension = readSpecCycle();
export const bundledSkills: BundledSkill[] = readBundledSkills();
