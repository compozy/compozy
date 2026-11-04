import { execFile } from "node:child_process";
import { lstat } from "node:fs/promises";
import { join } from "node:path";
import { promisify } from "node:util";

const execute = promisify(execFile);
const protectedDirectories = new Map<string, { identity: string; pending: Promise<void> | null }>();

/** chmod cannot protect tokens on Windows; new children must inherit a private DACL. */
export async function protectWindowsDirectory(path: string, refresh = false): Promise<void> {
  const metadata = await lstat(path, { bigint: true });
  const identity = `${metadata.dev}:${metadata.ino}:${metadata.birthtimeNs}`;
  let entry = protectedDirectories.get(path);
  if (entry?.identity !== identity || (refresh && !entry.pending)) {
    const pending = setPrivateACL(path).catch(error => {
      if (protectedDirectories.get(path)?.pending === pending) protectedDirectories.delete(path);
      throw error;
    });
    entry = { identity, pending };
    protectedDirectories.set(path, entry);
  }
  await entry.pending;
  if (protectedDirectories.get(path) === entry) entry.pending = null;
}

async function setPrivateACL(path: string): Promise<void> {
  const script = [
    "$ErrorActionPreference = 'Stop'",
    "$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().User",
    "$acl = [System.Security.AccessControl.DirectorySecurity]::new()",
    "$acl.SetOwner($user)",
    "$acl.SetAccessRuleProtection($true, $false)",
    "foreach ($sid in @($user.Value, 'S-1-5-18', 'S-1-5-32-544')) {" +
      "$identity = [System.Security.Principal.SecurityIdentifier]::new($sid);" +
      "$rule = [System.Security.AccessControl.FileSystemAccessRule]::new(" +
      "$identity, 'FullControl', 'ContainerInherit, ObjectInherit', 'None', 'Allow');" +
      "$acl.AddAccessRule($rule) }",
    "Set-Acl -LiteralPath $env:COMPOZY_PRIVATE_DIRECTORY -AclObject $acl",
  ].join("; ");
  await executeSecurityScript(script, { COMPOZY_PRIVATE_DIRECTORY: path });
}

/** Restrict the kernel pipe object before publishing its discovery file. */
export async function protectWindowsPipe(endpoint: string): Promise<void> {
  const script = [
    "$ErrorActionPreference = 'Stop'",
    "$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().User",
    "$pipe = [System.IO.Pipes.NamedPipeClientStream]::new('.', $env:COMPOZY_CONTROL_PIPE_NAME, [System.IO.Pipes.PipeAccessRights]::ChangePermissions, [System.IO.Pipes.PipeOptions]::None, [System.Security.Principal.TokenImpersonationLevel]::Anonymous, [System.IO.HandleInheritability]::None)",
    "try { $pipe.Connect(5000); " +
      "$acl = [System.IO.Pipes.PipeSecurity]::new(); " +
      "$acl.SetAccessRuleProtection($true, $false); " +
      "$acl.AddAccessRule([System.IO.Pipes.PipeAccessRule]::new($user, 'FullControl', 'Allow')); " +
      "$pipe.SetAccessControl($acl) } finally { $pipe.Dispose() }",
  ].join("; ");
  await executeSecurityScript(script, { COMPOZY_CONTROL_PIPE_NAME: endpoint.slice(9) });
}

async function executeSecurityScript(
  script: string,
  environment: Record<string, string>
): Promise<void> {
  await execute(
    join(
      process.env.SystemRoot || "C:\\Windows",
      "System32",
      "WindowsPowerShell",
      "v1.0",
      "powershell.exe"
    ),
    ["-NoProfile", "-NonInteractive", "-Command", script],
    {
      env: { ...process.env, ...environment },
      windowsHide: true,
      timeout: 10_000,
      maxBuffer: 64 * 1024,
    }
  );
}
