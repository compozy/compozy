import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse as parseYaml } from "yaml";
import { z } from "zod";
import { siteConfig } from "./site-config";

/**
 * What ships inside the binary, read from the same manifests the daemon reads.
 *
 * `extensions/dev-cycle` is enrolled by `devcycle.EnsureManagedInstall` at daemon boot with
 * `extensionpkg.SourceBundled` (`internal/daemon/boot_automation_bundles.go`). It therefore has no
 * catalog entry, no artifact URL, and no install command — publishing one would both misstate how it
 * arrives and collide with that managed install. The same is true of the skills embedded under
 * `skills/`: they are compiled in, not fetched from a registry.
 *
 * Counts are derived, never written down twice: the inventory is the directory listing plus the
 * manifest's own tool map.
 */

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const devCycleRoot = resolve(repoRoot, "extensions", "dev-cycle");
const bundledSkillsRoot = resolve(repoRoot, "skills");

const toolSchema = z.object({
  display_title: z.string().min(1),
  description: z.string().min(1),
  risk: z.enum(["read", "mutating", "destructive"]),
  read_only: z.boolean(),
  concurrency_safe: z.boolean(),
  visibility: z.enum(["model", "operator", "hidden"]),
});

const devCycleManifestSchema = z.object({
  extension: z.object({
    name: z.string().min(1),
    version: z.string().min(1),
    description: z.string().min(1),
    min_compozy_version: z.string().min(1),
  }),
  capabilities: z.object({ provides: z.array(z.string().min(1)).min(1) }),
  resources: z.object({
    skills: z.array(z.string().min(1)),
    loops: z.array(z.string().min(1)),
    agents: z.array(z.string().min(1)),
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

function listDirectories(root: string, subdirectories: string[]): string[] {
  const names: string[] = [];
  for (const subdirectory of subdirectories) {
    for (const entry of readdirSync(resolve(root, subdirectory), { withFileTypes: true })) {
      if (entry.isDirectory()) {
        names.push(entry.name);
      }
    }
  }
  return names.sort();
}

function readLoop(loopName: string): BundledLoop {
  const raw = readFileSync(resolve(devCycleRoot, "loops", loopName, "loop.yaml"), "utf8");
  const { meta } = loopMetaSchema.parse(parseYaml(raw));
  return {
    name: meta.name,
    description: meta.description,
    useWhen: meta.catalog?.use_when,
    category: meta.catalog?.category,
  };
}

function readDevCycle(): BundledExtension {
  const manifest = devCycleManifestSchema.parse(
    JSON.parse(readFileSync(resolve(devCycleRoot, "extension.json"), "utf8"))
  );
  const loopNames = listDirectories(devCycleRoot, manifest.resources.loops);
  return {
    name: manifest.extension.name,
    displayName: "Dev Cycle",
    version: manifest.extension.version,
    description: manifest.extension.description,
    minCompozyVersion: manifest.extension.min_compozy_version,
    provides: manifest.capabilities.provides,
    loops: loopNames.map(readLoop),
    skills: listDirectories(devCycleRoot, manifest.resources.skills),
    agents: listDirectories(devCycleRoot, manifest.resources.agents),
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
    repositoryUrl: `${siteConfig.repoUrl}/tree/main/extensions/dev-cycle`,
    statusCommand: `compozy extension status ${manifest.extension.name}`,
  };
}

/** `skills/embed.go` compiles every directory here into the binary. */
function readBundledSkills(): BundledSkill[] {
  const skills: BundledSkill[] = [];
  for (const entry of readdirSync(bundledSkillsRoot, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const raw = readFileSync(resolve(bundledSkillsRoot, entry.name, "SKILL.md"), "utf8");
    const name = raw.match(/^name:\s*(.+)$/m)?.[1]?.trim();
    const description = raw.match(/^description:\s*(.+)$/m)?.[1]?.trim();
    if (!name || !description) {
      throw new Error(`skills/${entry.name}/SKILL.md must declare name and description`);
    }
    skills.push({
      name,
      description,
      repositoryUrl: `${siteConfig.repoUrl}/tree/main/skills/${entry.name}`,
    });
  }
  return skills.sort((left, right) => left.name.localeCompare(right.name));
}

export const devCycleExtension: BundledExtension = readDevCycle();
export const bundledSkills: BundledSkill[] = readBundledSkills();
