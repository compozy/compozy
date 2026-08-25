/**
 * Showing an argument vector as one line, without lying about it.
 *
 * The tool sends `{command, args}` — a vector, not a string. Joining it with
 * spaces is only faithful while no argument contains a space, a newline, or a
 * shell metacharacter; past that, the line on screen is a *different* command
 * from the one being approved. `rm -rf "my files"` and `rm -rf my files` read
 * almost the same and do very different things.
 *
 * So each argument that is not plainly safe is quoted the way POSIX shells
 * quote: single quotes, with embedded single quotes spelled `'\''`. The result
 * is unambiguous and reversible, and it is a *representation* — never a
 * classification of what the command would do, which is the daemon's job.
 */

/** Characters a shell treats literally, so they need no quoting. */
const PLAIN = /^[A-Za-z0-9_@%+=:,./-]+$/;

function quote(argument: string): string {
  if (argument === "") return "''";
  if (PLAIN.test(argument)) return argument;
  return `'${argument.replaceAll("'", `'\\''`)}'`;
}

/**
 * Renders `command` plus its arguments as one unambiguous line.
 *
 * Returns null when any argument is not a string: a vector this client cannot
 * represent exactly must not be approved against an approximation of itself.
 */
export function formatTerminalArgv(command: string, args: unknown): string | null {
  if (args === undefined || args === null) return quote(command);
  if (!Array.isArray(args)) return null;
  const rendered: string[] = [];
  for (const argument of args) {
    if (typeof argument !== "string") return null;
    rendered.push(quote(argument));
  }
  return [quote(command), ...rendered].join(" ");
}
