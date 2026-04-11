import { createPromptDecoratorExtension } from "./extension.js";

const extension = createPromptDecoratorExtension();

extension.start().catch(error => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
