import type { SVGProps } from "react";

export function PiLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      className={className || "size-8"}
    >
      <title>Pi Logo</title>
      <rect width="24" height="24" rx="6" fill="#1A1A1A" />
      <path
        fill="#F5C54A"
        d="M5.5 8.6h13v1.9h-2.3l-.7 7.6c-.1 1 .4 1.4 1.1 1.4.4 0 .8-.2 1-.4l.4 1.6c-.5.4-1.3.7-2.2.7-1.8 0-2.8-1-2.6-2.9l.7-8H10l-1.1 8.8c-.2 1.4-1.2 2.1-2.4 2.1-.6 0-1.3-.2-1.7-.5l.5-1.7c.3.2.6.3.9.3.4 0 .7-.2.8-.9l1-8.1H5.5V8.6z"
      />
    </svg>
  );
}
