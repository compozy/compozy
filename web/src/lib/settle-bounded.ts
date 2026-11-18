/** Runs `task` over `inputs`, at most `limit` at once, and keeps each outcome at its input's index. */
export function settleBounded<Input, Output>(
  inputs: readonly Input[],
  limit: number,
  task: (input: Input) => Promise<Output>
): Promise<PromiseSettledResult<Output>[]> {
  const outcomes: PromiseSettledResult<Output>[] = [];
  let next = 0;
  const worker = (): Promise<void> => {
    const index = next;
    next += 1;
    if (index >= inputs.length) return Promise.resolve();
    return task(inputs[index]!)
      .then(
        value => {
          outcomes[index] = { status: "fulfilled", value };
        },
        (reason: unknown) => {
          outcomes[index] = { status: "rejected", reason };
        }
      )
      .then(worker);
  };
  const workers = Array.from({ length: Math.min(limit, inputs.length) }, () => worker());
  return Promise.all(workers).then(() => outcomes);
}
