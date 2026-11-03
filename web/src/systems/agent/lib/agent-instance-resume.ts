import type { AgentInstancePayload } from "../types";

export interface AgentResumeFailure {
  instance: AgentInstancePayload;
  message: string;
}

/** Thrown when one or more resumes were refused; each refused agent keeps its daemon reason. */
export class AgentResumeError extends Error {
  readonly failures: readonly AgentResumeFailure[];

  constructor(failures: readonly AgentResumeFailure[]) {
    super(agentResumeFailureCopy(failures));
    this.name = "AgentResumeError";
    this.failures = failures;
  }
}

/**
 * Resume one at a time, in the order given: the daemon admits resumes against the active-agent
 * cap, so a fixed order decides which agents get the last slots. Every resume is attempted.
 */
export async function resumeInOrder(
  instances: readonly AgentInstancePayload[],
  resume: (instance: AgentInstancePayload) => Promise<unknown>
): Promise<void> {
  const failures: AgentResumeFailure[] = [];
  for (const instance of instances) {
    try {
      // react-doctor-disable-next-line react-doctor/async-await-in-loop -- one resume at a time is the point: the daemon admits each against the active-agent cap, so a fixed order decides which agents get the last slots.
      await resume(instance);
    } catch (error) {
      failures.push({
        instance,
        message: error instanceof Error ? error.message : "It couldn't be resumed.",
      });
    }
  }
  if (failures.length > 0) throw new AgentResumeError(failures);
}

function nameList(names: readonly string[]) {
  if (names.length <= 1) return names[0] ?? "";
  return `${names.slice(0, -1).join(", ")} and ${names.at(-1)}`;
}

/** "Ops Lead and Release Watch didn't resume. <daemon reason>" */
export function agentResumeFailureCopy(failures: readonly AgentResumeFailure[]) {
  const names = nameList(failures.map(failure => failure.instance.label));
  const reasons = new Set<string>();
  for (const failure of failures) {
    const reason = failure.message.trim();
    if (reason) reasons.add(reason);
  }
  return [`${names} didn't resume.`, ...reasons].join(" ");
}
