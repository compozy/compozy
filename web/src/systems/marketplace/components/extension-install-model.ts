import type { ExtensionInstallRequest } from "../types";

export type ExtensionInstallSource = "local_path" | "github" | "git";

export const EXTENSION_INSTALL_SOURCES: readonly {
  value: ExtensionInstallSource;
  label: string;
  hint: string;
  refLabel: string;
  refPlaceholder: string;
}[] = [
  {
    value: "local_path",
    label: "Local path",
    hint: "An absolute path to a built extension directory on this machine.",
    refLabel: "Directory path",
    refPlaceholder: "/Users/you/src/hello/dist/gen-a1b2c3",
  },
  {
    value: "github",
    label: "GitHub release",
    hint: "An owner/repo release. Add @tag to pin one release.",
    refLabel: "Repository",
    refPlaceholder: "acme/hello",
  },
  {
    value: "git",
    label: "Git URL",
    hint: "A public HTTPS repository URL. Add a branch, tag, or commit in Version.",
    refLabel: "Repository URL",
    refPlaceholder: "https://git.example.com/acme/hello.git",
  },
];

export interface ExtensionInstallForm {
  source: ExtensionInstallSource;
  ref: string;
  version: string;
  asset: string;
  allowUnverified: boolean;
}

export type ExtensionInstallFieldError = Partial<Record<"ref" | "version" | "asset", string>>;

export function createExtensionInstallForm(): ExtensionInstallForm {
  return { allowUnverified: false, asset: "", ref: "", source: "local_path", version: "" };
}

/** Performs the form's early syntax checks; the daemon owns destination and DNS policy. */
export function validateExtensionInstallForm(
  form: ExtensionInstallForm
): ExtensionInstallFieldError {
  const ref = form.ref.trim();
  if (ref === "") return { ref: "A source reference is required." };
  if (form.source === "local_path") return validateLocalPath(ref);
  if (form.source === "github") return validateGitHubRef(ref);
  return validateGitRef(ref);
}

function validateLocalPath(ref: string): ExtensionInstallFieldError {
  const absolute = ref.startsWith("/") || /^[A-Za-z]:[\\/]/.test(ref);
  if (!absolute) {
    return { ref: "Use an absolute path — CompozyOS resolves it on its own machine." };
  }
  return {};
}

function validateGitHubRef(ref: string): ExtensionInstallFieldError {
  const at = ref.lastIndexOf("@");
  if (
    at === 0 ||
    at === ref.length - 1 ||
    (at > 0 && ref.indexOf("@") !== at) ||
    (at > 0 && ref.slice(at + 1).trim() !== ref.slice(at + 1))
  ) {
    return { ref: "Use owner/repo, optionally pinned with @tag." };
  }
  const repository = at > 0 ? ref.slice(0, at) : ref;
  const parts = repository.split("/");
  if (
    parts.length !== 2 ||
    parts.some(part => part.trim() === "" || part.trim() !== part || part.includes("@"))
  ) {
    return { ref: "Use owner/repo, optionally pinned with @tag." };
  }
  return {};
}

function validateGitRef(ref: string): ExtensionInstallFieldError {
  const repository = splitGitDistributionRef(ref);
  let parsed: URL;
  try {
    parsed = new URL(repository);
  } catch {
    return {
      ref: "Use a public HTTPS repository URL, such as https://git.example.com/acme/hello.git.",
    };
  }
  if (parsed.protocol !== "https:") {
    return {
      ref: "Use a public HTTPS repository URL, such as https://git.example.com/acme/hello.git.",
    };
  }
  const authority = gitURLAuthority(repository);
  if (parsed.username !== "" || parsed.password !== "" || authority.includes("@")) {
    return { ref: "Remove credentials from the URL. Credentials in Git URLs are not accepted." };
  }
  if (
    parsed.search !== "" ||
    parsed.hash !== "" ||
    repository.includes("?") ||
    repository.includes("#")
  ) {
    return { ref: "Remove the query or fragment. Put the Git ref in the Version field." };
  }
  if (
    parsed.hostname === "" ||
    parsed.hostname.endsWith(".") ||
    /\P{ASCII}/u.test(authority) ||
    parsed.pathname === "/" ||
    parsed.port === "0"
  ) {
    return { ref: "Add a valid host and repository path to the HTTPS URL." };
  }
  return {};
}

function gitURLAuthority(value: string): string {
  const start = value.indexOf("://") + 3;
  const end = value.indexOf("/", start);
  return value.slice(start, end < 0 ? undefined : end);
}

function splitGitDistributionRef(value: string): string {
  const index = value.lastIndexOf("@");
  if (index <= 0 || index === value.length - 1) return value;
  const scheme = value.indexOf("://");
  if (scheme >= 0) {
    const hostEnd = value.slice(scheme + 3).indexOf("/");
    if (hostEnd < 0 || index < scheme + 3 + hostEnd) return value;
  }
  return value.slice(0, index).trim();
}

export function buildExtensionInstallRequest(form: ExtensionInstallForm): ExtensionInstallRequest {
  const version = form.version.trim();
  const asset = form.asset.trim();
  return {
    ...(form.allowUnverified ? { allow_unverified: true } : {}),
    ...(asset !== "" && form.source === "github" ? { asset } : {}),
    ref: form.ref.trim(),
    source: form.source,
    ...(version !== "" ? { version } : {}),
  };
}
