import { renderLandingOG } from "@/lib/og/templates/landing";

export const size = {
  width: 1200,
  height: 630,
};

export const contentType = "image/png";
export const dynamic = "force-static";

export default async function Image() {
  return renderLandingOG();
}
