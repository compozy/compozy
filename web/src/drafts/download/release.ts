/** A real snapshot of the v0.5.5 GitHub release. Production reads this from the release API. */
export type Platform = "macos" | "linux" | "windows";
export type MacArch = "arm64" | "x64";

const BASE = "https://github.com/compozy/compozy/releases/download/v0.5.5";
export const LATEST_URL = "https://github.com/compozy/compozy/releases/latest";

export const RELEASE = {
  version: "v0.5.5",
  date: "Nov 9, 2026",
  notes: "https://github.com/compozy/compozy/releases/tag/v0.5.5",
  checksums: `${BASE}/checksums.txt`,
  signature: `${BASE}/checksums.txt.sigstore.json`,
};

export interface Artifact {
  name: string;
  detail: string;
  file: string;
  bytes: number;
}

function artifact(name: string, detail: string, file: string, bytes: number): Artifact {
  return { name, detail, file, bytes };
}

export const DESKTOP = {
  macArm: artifact("Apple silicon", "DMG", "CompozyOS-0.5.5-mac-arm64.dmg", 225178493),
  macIntel: artifact("Intel", "DMG", "CompozyOS-0.5.5-mac-x64.dmg", 238191261),
  appImage: artifact("x64", "AppImage", "CompozyOS-0.5.5-linux-x64.AppImage", 219117499),
  deb: artifact("x64", "deb", "CompozyOS-0.5.5-linux-x64.deb", 193608260),
};

export const WINDOWS_COMMAND = artifact("x64", "ZIP", "compozy_windows_x86_64.zip", 62159815);

export const COMMAND_PACKAGES: ReadonlyArray<readonly [string, readonly Artifact[]]> = [
  [
    "macOS",
    [
      artifact("Apple silicon", "tar.gz", "compozy_darwin_arm64.tar.gz", 59231199),
      artifact("Intel", "tar.gz", "compozy_darwin_x86_64.tar.gz", 62048135),
    ],
  ],
  [
    "Linux",
    [
      artifact("x64", "tar.gz", "compozy_linux_x86_64.tar.gz", 61536683),
      artifact("ARM64", "tar.gz", "compozy_linux_arm64.tar.gz", 57288219),
      artifact("x64", "deb", "compozy_0.5.5_amd64.deb", 61512070),
      artifact("ARM64", "deb", "compozy_0.5.5_arm64.deb", 57262910),
      artifact("x64", "rpm", "compozy-0.5.5-1.x86_64.rpm", 61328853),
      artifact("ARM64", "rpm", "compozy-0.5.5-1.aarch64.rpm", 56946522),
    ],
  ],
];

export function artifactUrl(item: Artifact) {
  return `${BASE}/${item.file}`;
}

export function megabytes(bytes: number) {
  return `${Math.round(bytes / 1048576)} MB`;
}

export interface Offer {
  cta: string;
  item: Artifact;
  platformLabel: string;
}

export function offerFor(platform: Platform, arch: MacArch): Offer {
  if (platform === "linux")
    return { cta: "Download for Linux", item: DESKTOP.appImage, platformLabel: "Linux" };
  if (platform === "windows")
    return { cta: "Download the command", item: WINDOWS_COMMAND, platformLabel: "Windows" };
  return {
    cta: "Download for macOS",
    item: arch === "arm64" ? DESKTOP.macArm : DESKTOP.macIntel,
    platformLabel: "macOS",
  };
}

export const INSTALLS = [
  {
    id: "Installer",
    command: "curl -fsSL https://compozy.com/install.sh | sh",
    note: "macOS and Linux. Checks the release signature before it installs.",
  },
  {
    id: "Homebrew",
    command: "brew install compozy/compozy/compozy",
    note: "macOS and Linux. Homebrew owns the updates.",
  },
  { id: "npm", command: "npm install -g @compozy/cli", note: "npm owns the updates." },
  {
    id: "Go",
    command: "go install github.com/compozy/compozy@v0.5.5",
    note: "Pinned to this release.",
  },
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
