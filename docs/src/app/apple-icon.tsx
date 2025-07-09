import { readFileSync } from "fs";
import { ImageResponse } from "next/og";
import { join } from "path";

// Image metadata for Apple devices
export const size = {
  width: 512,
  height: 512,
};
export const contentType = "image/png";

// Image generation
export default async function AppleIcon() {
  // Read the symbol.png file from the public directory
  const imageBuffer = readFileSync(join(process.cwd(), "public", "symbol.png"));
  const base64Image = `data:image/png;base64,${imageBuffer.toString("base64")}`;

  return new ImageResponse(
    (
      // ImageResponse JSX element
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "transparent",
        }}
      >
        {/* Using img element instead of background-image */}
        <img
          src={base64Image}
          alt="Compozy Icon"
          style={{
            width: "100%",
            height: "100%",
            objectFit: "contain",
          }}
        />
      </div>
    ),
    // ImageResponse options
    {
      ...size,
    }
  );
}
