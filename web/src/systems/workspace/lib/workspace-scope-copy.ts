import type { Focus } from "./destination";

/** Destination vocabulary: All projects (the globe), User, and projects. Never "Global". */
export const DESTINATION_COPY = {
  allProjects: "All projects",
  /** Text mark for All projects where only a monogram fits; the chip draws a stack icon. */
  allProjectsMonogram: "∗",
  user: "User",
  userMonogram: "~",
  liveOn: "All projects on",
  liveOff: "All projects off",
  skipOnboarding: "Skip — use my home folder",
} as const;

/** Globe tooltip while All projects is on: where turning it off returns. */
export function allProjectsTooltipOn(focusName: string): string {
  return `Back to ${focusName}`;
}

/** Where a new item lands: inside a project, or in User outside any project. */
export type DestinationKind = "project" | "user";

/** Display name of a destination: the project's name, or "User". */
export function destinationLabel(
  kind: DestinationKind,
  workspaceName: string | undefined | null
): string {
  if (kind === "user") return DESTINATION_COPY.user;
  const name = workspaceName?.trim();
  return name && name.length > 0 ? name : "project";
}

/** Display name of a focus: the project's name, or "User". */
export function focusLabel(focus: Focus, workspaceName: string | undefined | null): string {
  return destinationLabel(focus.kind, workspaceName);
}
