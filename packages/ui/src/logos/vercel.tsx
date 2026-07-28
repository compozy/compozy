import type { SVGProps } from "react";

export function VercelLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 256 222"
      width="256"
      height="222"
      xmlns="http://www.w3.org/2000/svg"
      preserveAspectRatio="xMidYMid"
      className={className || "size-8"}
    >
      <title>Vercel</title>
      <path fill="currentColor" d="m128 0 128 221.705H0z" />
    </svg>
  );
}
