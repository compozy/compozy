import { createLifecycleObserverExtension } from "./extension.js";

const extension = createLifecycleObserverExtension();

extension.start().catch(error => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
