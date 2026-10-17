import type { SVGProps } from "react";

// Official Antigravity mark from Google's ACP registry
// (https://cdn.agentclientprotocol.com/registry/v1/latest/antigravity-acp.svg,
// entry `antigravity-acp`, publisher Google LLC). Geometry preserved; the single
// path uses `currentColor` so the mark follows the icon's text color in either
// theme instead of carrying fixed brand ink.
export function AntigravityLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 16 16"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <title>Antigravity</title>
      <path
        d="M14.1452 14.6818C14.9937 15.3182 16.2664 14.894 15.0997 13.7273C11.5998 10.3333 12.3421 1 7.99366 1C3.64518 1 4.3876 10.3333 0.887603 13.7273C-0.385123 15 0.993664 15.3182 1.84215 14.6818C5.13002 12.4545 4.9179 8.5303 7.99366 8.5303C11.0694 8.5303 10.8573 12.4545 14.1452 14.6818Z"
        fill="currentColor"
      />
    </svg>
  );
}
