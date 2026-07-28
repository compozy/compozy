import { getLLMText } from "@/lib/get-llm-text";
import { protocolDocs, runtimeDocs } from "@/lib/source";

export const dynamic = "force-static";
export const revalidate = false;

const PAGES = [...runtimeDocs.getPages(), ...protocolDocs.getPages()];

export async function GET() {
  const sections = await Promise.all(PAGES.map(page => getLLMText(page)));
  return new Response(sections.join("\n\n---\n\n"), {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
