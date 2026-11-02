/** The next face in a seed's line; the slug stays untouched. */
export function nextAgentAvatarSeed(seed: string) {
  const [base = seed, count = "0"] = seed.split("~");
  return `${base}~${Number(count) + 1}`;
}
