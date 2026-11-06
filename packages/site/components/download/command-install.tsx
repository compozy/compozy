import { LandingCodeBlock } from "@/components/landing/primitives/code-block";
import { CtaButton } from "@/components/landing/primitives/cta-button";
import { SectionFrame } from "@/components/landing/primitives/section-frame";
import { SectionHeader } from "@/components/landing/primitives/section-header";
import {
  HOMEBREW_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
} from "@/lib/release/install-commands";

export function CommandInstall() {
  return (
    <SectionFrame id="command" background="canvas" padY="md" className="border-b border-line">
      <div className="grid gap-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:items-start">
        <div>
          <SectionHeader
            eyebrow="Prefer the terminal?"
            title="Install the compozy command."
            description="Run the same runtime headless, on a server, or from scripts. The desktop app and the command share one daemon and one ~/.compozy home, so you can use both."
          />
          <div className="mt-8">
            <CtaButton href="/docs/getting-started/installation" variant="ghost">
              Read the installation guide
            </CtaButton>
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <LandingCodeBlock
            code={VERIFIED_INSTALLER_COMMAND}
            caption="Verified installer · macOS and Linux · Sigstore provenance"
            shell
          />
          <LandingCodeBlock
            code={HOMEBREW_INSTALL_COMMAND}
            caption="Homebrew · macOS and Linux"
            shell
          />
          <p className="text-sm leading-relaxed text-muted-foreground">
            npm, a pinned Go release, and Linux packages are covered in the installation guide.
          </p>
        </div>
      </div>
    </SectionFrame>
  );
}
