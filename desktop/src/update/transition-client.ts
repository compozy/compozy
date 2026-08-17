import { execFile } from "node:child_process";

import {
  parseUpdateOperation,
  type AppOperationPhase,
  type UpdateOperation,
} from "./operation-contract";

export interface TransitionIdentity {
  readonly operationId: string;
  readonly revision: number;
  readonly executorGeneration: string;
}

export interface PhaseTransition extends TransitionIdentity {
  readonly phase: AppOperationPhase;
  readonly percent: number;
  readonly lastError?: string;
  readonly outcome?: string;
  readonly watchdogDeadline?: Date | "clear";
  readonly incrementFailures?: boolean;
  readonly resetFailures?: boolean;
}

export interface UpdateTransitionPort {
  acquire(identity: TransitionIdentity): Promise<UpdateOperation>;
  recover(identity: TransitionIdentity): Promise<UpdateOperation>;
  renew(identity: TransitionIdentity): Promise<UpdateOperation>;
  fence(identity: TransitionIdentity): Promise<UpdateOperation>;
  progress(identity: TransitionIdentity, percent: number): Promise<UpdateOperation>;
  phase(transition: PhaseTransition): Promise<UpdateOperation>;
  verifyArtifact(
    identity: TransitionIdentity,
    attemptId: string,
    artifactPath: string
  ): Promise<UpdateOperation>;
}

const COMMAND_TIMEOUT_MS = 15_000;
const MAXIMUM_OUTPUT_BYTES = 1024 * 1024;

export class UpdateTransitionClient implements UpdateTransitionPort {
  readonly #runtimePath: string;

  constructor(runtimePath: string) {
    this.#runtimePath = runtimePath;
  }

  async acquire(identity: TransitionIdentity): Promise<UpdateOperation> {
    return await this.#run("acquire", identity);
  }

  async recover(identity: TransitionIdentity): Promise<UpdateOperation> {
    return await this.#run("recover", identity);
  }

  async renew(identity: TransitionIdentity): Promise<UpdateOperation> {
    return await this.#run("renew", identity);
  }

  async fence(identity: TransitionIdentity): Promise<UpdateOperation> {
    return await this.#run("fence", identity);
  }

  async progress(identity: TransitionIdentity, percent: number): Promise<UpdateOperation> {
    return await this.#run("progress", identity, ["--percent", String(percent)]);
  }

  async phase(transition: PhaseTransition): Promise<UpdateOperation> {
    const extra = ["--phase", transition.phase, "--percent", String(transition.percent)];
    if (transition.lastError) extra.push("--last-error", transition.lastError);
    if (transition.outcome) extra.push("--outcome", transition.outcome);
    if (transition.watchdogDeadline) {
      extra.push(
        "--watchdog-deadline",
        transition.watchdogDeadline === "clear"
          ? "clear"
          : transition.watchdogDeadline.toISOString()
      );
    }
    if (transition.incrementFailures) extra.push("--increment-failures");
    if (transition.resetFailures) extra.push("--reset-failures");
    return await this.#run("phase", transition, extra);
  }

  async verifyArtifact(
    identity: TransitionIdentity,
    attemptId: string,
    artifactPath: string
  ): Promise<UpdateOperation> {
    return await this.#run("verify-artifact", identity, [
      "--attempt-id",
      attemptId,
      "--artifact-path",
      artifactPath,
    ]);
  }

  async #run(
    action: string,
    identity: TransitionIdentity,
    extra: readonly string[] = []
  ): Promise<UpdateOperation> {
    const arguments_ = [
      "daemon",
      "app-update-transition",
      "--action",
      action,
      "--operation-id",
      identity.operationId,
      "--expected-revision",
      String(identity.revision),
      "--executor-generation",
      identity.executorGeneration,
      ...extra,
    ];
    const stdout = await executeTransition(this.#runtimePath, arguments_);
    let value: unknown;
    try {
      value = JSON.parse(stdout);
    } catch (error) {
      throw new Error("The update transition returned invalid JSON.", { cause: error });
    }
    return parseUpdateOperation(value);
  }
}

async function executeTransition(command: string, arguments_: readonly string[]): Promise<string> {
  return await new Promise((resolve, reject) => {
    execFile(
      command,
      [...arguments_],
      { timeout: COMMAND_TIMEOUT_MS, maxBuffer: MAXIMUM_OUTPUT_BYTES, encoding: "utf8" },
      (error, stdout, stderr) => {
        if (error) {
          const detail = stderr.trim();
          reject(new Error(detail || "The app update transition failed.", { cause: error }));
          return;
        }
        resolve(stdout);
      }
    );
  });
}
