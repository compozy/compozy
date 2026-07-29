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
    hint: "An absolute path to a built extension directory on this daemon's host.",
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
    hint: "A public git repository URL cloned at the requested ref.",
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

/** Mirrors the daemon's per-source ref grammar so a malformed ref never reaches the API. */
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
    return { ref: "Use an absolute path — the daemon resolves it on its own host." };
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
  if (!/^(https?|git|ssh):\/\//.test(ref) && !ref.startsWith("git@")) {
    return { ref: "Use a git URL (https://, git://, ssh://, or git@host:owner/repo)." };
  }
  if (/^https?:\/\/[^/]*@/.test(ref)) {
    return { ref: "Remove credentials from the URL — the daemon uses ambient git credentials." };
  }
  return {};
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
