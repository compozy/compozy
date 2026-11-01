import { Lock } from "@compozy/ui/icons";

/** The line above the secrets: what the vault keeps and what it never shows again. */
export function VaultSecurityNote() {
  return (
    <p
      className="mb-3 flex items-center gap-2 text-eyebrow text-subtle"
      data-testid="vault-page-sec-note"
    >
      <Lock aria-hidden="true" className="size-3 shrink-0 text-faint" />
      Values are encrypted. You can't view a secret after you save it.
    </p>
  );
}
