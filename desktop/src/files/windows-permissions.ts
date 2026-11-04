import { execFile } from "node:child_process";
import { join } from "node:path";
import { promisify } from "node:util";

const execute = promisify(execFile);
const protectedDirectories = new Map<string, Promise<void>>();

/** chmod cannot protect tokens on Windows; new children must inherit a private DACL. */
export async function protectWindowsDirectory(path: string): Promise<void> {
  let pending = protectedDirectories.get(path);
  if (!pending) {
    pending = setPrivateACL(path).catch(error => {
      protectedDirectories.delete(path);
      throw error;
    });
    protectedDirectories.set(path, pending);
  }
  await pending;
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
      env: { ...process.env, COMPOZY_PRIVATE_DIRECTORY: path },
      windowsHide: true,
      timeout: 10_000,
      maxBuffer: 64 * 1024,
    }
  );
}
