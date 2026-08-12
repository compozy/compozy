export interface TaskQueryHookOptions {
  enabled?: boolean;
  /**
   * Refetch override modes: omit it to retain the query factory interval, use
   * `false` to disable polling, or provide milliseconds to replace the interval.
   */
  refetchIntervalMs?: number | false;
}

export function withTaskQueryHookOptions<T extends object>(
  queryOptions: T,
  hookOptions: TaskQueryHookOptions
) {
  return {
    ...queryOptions,
    ...(hookOptions.refetchIntervalMs === undefined
      ? {}
      : { refetchInterval: hookOptions.refetchIntervalMs }),
  };
}
