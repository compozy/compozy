import { readdirSync, readFileSync, statSync } from "node:fs";
import { basename, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse as parseYaml } from "yaml";
import { z } from "zod";
import { siteConfig } from "./site-config";

/** Read built-in resources from the same manifests enrolled at daemon boot. */
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const bundledSkillsRoot = resolve(repoRoot, "skills");
const bundledDefinitions = [
  { name: "spec-cycle", displayName: "Spec Cycle" },
  { name: "open-design", displayName: "Open Design" },
];

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

const bundledManifestSchema = z.object({
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
  path: string;
  /** The only command that applies: it is already installed, so you can only inspect it. */
  statusCommand: string;
}

export interface BundledSkill {
  name: string;
  description: string;
  repositoryUrl: string;
}

function listResourceFiles(root: string, paths: string[], filename: string): string[] {
  return paths
    .flatMap(path => {
      const resourcePath = resolve(root, path);
      if (statSync(resourcePath).isFile()) return [resourcePath];
      return readdirSync(/* turbopackIgnore: true */ resourcePath, { withFileTypes: true }).flatMap(
        entry => (entry.isDirectory() ? [resolve(resourcePath, entry.name, filename)] : [])
      );
    })
    .sort();
}

function readLoop(path: string): BundledLoop {
  const { meta } = loopMetaSchema.parse(parseYaml(readFileSync(path, "utf8")));
  return {
    name: meta.name,
    description: meta.description,
    useWhen: meta.catalog?.use_when,
    category: meta.catalog?.category,
  };
}

function readBundledExtension(definition: (typeof bundledDefinitions)[number]): BundledExtension {
  const root = resolve(repoRoot, "extensions", definition.name);
  const manifest = bundledManifestSchema.parse(
    JSON.parse(readFileSync(resolve(root, "extension.json"), "utf8"))
  );
  return {
    name: manifest.extension.name,
    displayName: definition.displayName,
    version: manifest.extension.version,
    description: manifest.extension.description,
    minCompozyVersion: manifest.extension.min_compozy_version,
    provides: manifest.capabilities.provides,
    loops: listResourceFiles(root, manifest.resources.loops, "loop.yaml").map(readLoop),
    skills: listResourceFiles(root, manifest.resources.skills, "SKILL.md")
      .map(path => parseBundledSkillFrontmatter(readFileSync(path, "utf8")).name)
      .sort(),
    agents: listResourceFiles(root, manifest.resources.agents, "AGENT.md").map(path =>
      basename(dirname(path))
    ),
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
    repositoryUrl: `${siteConfig.repoUrl}/tree/${siteConfig.repoBranch}/extensions/${definition.name}`,
    path: `/marketplace/bundled/${manifest.extension.name}`,
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
  for (const entry of readdirSync(/* turbopackIgnore: true */ bundledSkillsRoot, {
    withFileTypes: true,
  })) {
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

export const bundledExtensions: BundledExtension[] = bundledDefinitions.map(readBundledExtension);
export const bundledSkills: BundledSkill[] = readBundledSkills();
