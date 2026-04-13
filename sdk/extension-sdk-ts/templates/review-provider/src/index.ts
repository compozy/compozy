import { createReviewProviderExtension } from "./extension.js";

const extension = createReviewProviderExtension();

extension.start().catch(error => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
