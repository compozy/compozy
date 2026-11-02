/** `+3 PRs` in the card's meta row, in the branch's place. */
export function SessionBoardLinkedWorkCount({ label }: { label: string }) {
  return (
    <span className="min-w-0 shrink-0 font-sans text-eyebrow tracking-eyebrow text-muted-foreground">
      {label}
    </span>
  );
}
