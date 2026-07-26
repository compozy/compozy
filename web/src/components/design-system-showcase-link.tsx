import { Button } from "@agh/ui";

const DESIGN_MD_BASE = "https://github.com/compozy/agh/blob/main/DESIGN.md";

export function DesignSystemShowcaseLink() {
  return (
    <Button
      nativeButton={false}
      size="sm"
      variant="outline"
      render={
        <a
          aria-label="Open DESIGN.md"
          data-testid="showcase-open-design-md"
          href={DESIGN_MD_BASE}
          target="_blank"
          rel="noreferrer"
        />
      }
    >
      Open DESIGN.md
    </Button>
  );
}
