export interface TaskQueryHookOptions {
  enabled?: boolean;
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
