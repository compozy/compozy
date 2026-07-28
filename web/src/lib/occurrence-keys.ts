export interface OccurrenceKeyed<T> {
  item: T;
  key: string;
}

export function withOccurrenceKeys<T>(
  items: readonly T[],
  identify: (item: T) => string
): OccurrenceKeyed<T>[] {
  const counts = new Map<string, number>();
  return items.map(item => {
    const identity = identify(item);
    const occurrence = counts.get(identity) ?? 0;
    counts.set(identity, occurrence + 1);
    return { item, key: `${identity.length}:${identity}:${occurrence}` };
  });
}
