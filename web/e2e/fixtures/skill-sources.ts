import { lstat, mkdir, readlink, rm, symlink, writeFile } from "node:fs/promises";
import path from "node:path";

/**
 * Filesystem fixtures for skill sources.
 *
 * The daemon reads real folders, so these helpers write real ones under the
 * runtime's operator home. Everything here manipulates the disk the way another
 * tool or a person would — deleting a link, or dropping a foreign one in its
 * place — because that is exactly the state the expose panel has to report.
 */

export type SkillSourceConvention = "agents" | "claude";

/** Where a preset's user-level root lives under a given home. */
export function providerSkillsRoot(operatorHomeDir: string, source: SkillSourceConvention): string {
  return path.join(operatorHomeDir, `.${source}`, "skills");
}

/** Where an exposed skill's link lands for a preset target. */
export function exposeLinkPath(
  operatorHomeDir: string,
  target: SkillSourceConvention,
  skillName: string
): string {
  return path.join(providerSkillsRoot(operatorHomeDir, target), skillName);
}

export async function writeSkillDefinition(
  skillDir: string,
  name: string,
  description: string
): Promise<void> {
  await mkdir(skillDir, { recursive: true });
  const document = [
    "---",
    `name: ${JSON.stringify(name)}`,
    `description: ${JSON.stringify(description)}`,
    "---",
    "",
    `# ${name}`,
    "",
    description,
    "",
  ].join("\n");
  await writeFile(path.join(skillDir, "SKILL.md"), document, { encoding: "utf8", mode: 0o600 });
}

/** Seeds a skill into a preset's user-level folder, the way another tool would. */
export async function seedProviderSkill(
  operatorHomeDir: string,
  source: SkillSourceConvention,
  name: string,
  description = `${name} fixture skill`
): Promise<string> {
  const skillDir = path.join(providerSkillsRoot(operatorHomeDir, source), name);
  await writeSkillDefinition(skillDir, name, description);
  return skillDir;
}

/** Seeds a folder the operator would register by hand as a custom source. */
export async function seedCustomSkillRoot(
  rootDir: string,
  names: readonly string[]
): Promise<string> {
  for (const name of names) {
    await writeSkillDefinition(path.join(rootDir, name), name, `${name} custom-root skill`);
  }
  return rootDir;
}

export async function pathExists(target: string): Promise<boolean> {
  try {
    await lstat(target);
    return true;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return false;
    throw error;
  }
}

export async function readLinkTarget(target: string): Promise<string | null> {
  try {
    return await readlink(target);
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT" || code === "EINVAL") return null;
    throw error;
  }
}

/** Deletes an expose link the way a person cleaning up their home folder would. */
export async function deleteExposeLink(linkPath: string): Promise<void> {
  await rm(linkPath, { recursive: false });
}

/**
 * Replaces an expose link with one pointing somewhere else, so the daemon sees a
 * link at our path whose destination is not ours — a foreign conflict.
 */
export async function replaceWithForeignLink(
  linkPath: string,
  foreignTarget: string
): Promise<void> {
  await mkdir(foreignTarget, { recursive: true });
  await rm(linkPath, { recursive: false });
  await symlink(foreignTarget, linkPath);
}
