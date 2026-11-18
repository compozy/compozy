/** Download page copy: terminal install methods and the first-run steps per platform. */
import {
  HOMEBREW_INSTALL_COMMAND,
  NPM_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
  goInstallCommand,
} from "@/lib/release/install-commands";
import type { Platform } from "@/lib/release/download-offer";

export interface InstallMethod {
  id: string;
  command: string;
  note: string;
}

/** `tag` pins the Go command to the release on the page; unknown releases install the latest. */
export function installMethods(tag: string | null): InstallMethod[] {
  return [
    {
      id: "Installer",
      command: VERIFIED_INSTALLER_COMMAND,
      note: "macOS and Linux. Checks the release signature before it installs.",
    },
    {
      id: "Homebrew",
      command: HOMEBREW_INSTALL_COMMAND,
      note: "macOS and Linux. Homebrew owns the updates.",
    },
    { id: "npm", command: NPM_INSTALL_COMMAND, note: "npm owns the updates." },
    tag
      ? { id: "Go", command: goInstallCommand(tag), note: "Pinned to this release." }
      : { id: "Go", command: goInstallCommand("latest"), note: "The latest release." },
  ];
}

export interface Step {
  title: string;
  body: string;
}

const START: Step = {
  title: "Start a session",
  body: "Pick a project and the agent you already use. Your sign-in stays with that agent.",
};
const OPEN: Step = {
  title: "Open the app",
  body: "It starts the runtime it carries inside. A clean first run doesn't need a network connection.",
};

export const STEPS: Record<Platform, readonly Step[]> = {
  macos: [
    { title: "Drag it to Applications", body: "Open the notarized DMG and drag CompozyOS across." },
    OPEN,
    START,
  ],
  linux: [
    {
      title: "Run the AppImage",
      body: "Or install the deb package. Verify either one against the release checksums.",
    },
    OPEN,
    START,
  ],
  windows: [
    {
      title: "Unzip the command",
      body: "There's no desktop app for Windows yet. Put compozy somewhere on your PATH.",
    },
    {
      title: "Run the setup",
      body: "One interactive step in your terminal prepares the local runtime.",
    },
    START,
  ],
};
