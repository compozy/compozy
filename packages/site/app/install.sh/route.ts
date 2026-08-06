import { renderInstallScript } from "@/lib/install/render-install-script";
import { INSTALL_SCRIPT_TEMPLATE } from "@/lib/install/template-content";
import { getCurrentRelease } from "@/lib/release/current-release";

export const revalidate = 300;

export async function GET(): Promise<Response> {
  const release = await getCurrentRelease();
  return new Response(renderInstallScript(INSTALL_SCRIPT_TEMPLATE, release.tag), {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=300, must-revalidate",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
