/** Download page copy: terminal install methods and the first-run steps per platform. */
import {
  HOMEBREW_INSTALL_COMMAND,
  NPM_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
} from "@/lib/release/install-commands";
import type { Platform } from "@/lib/release/download-offer";

export interface InstallMethod {
  id: string;
  command: string;
  note: string;
}

/** Official terminal install methods (ADR-014 §3; source builds live in the contributor guide). */
export const INSTALL_METHODS: readonly InstallMethod[] = [
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
];

export interface Step {
  title: string;
  body: string;
}

const START: Step = {
  title: "Start a session",
  body: "Pick a project and the agent you already use. Your sign-in stays with that agent.",
};
const OPEN: Step = {
  title: "Open the app and sign in",
  body: "It starts the bundled runtime and asks you to sign in once with Google or GitHub. Local work continues offline after that.",
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
      title: "Run setup and sign in",
      body: "Run compozy install and compozy account login to prepare the local runtime. First sign-in needs the internet; later use works offline.",
    },
    START,
  ],
};
