import type { SVGProps } from "react";

export function KiroLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      className={className || "size-8"}
    >
      <title>Kiro Logo</title>
      <circle cx="12" cy="12" r="11" fill="#7A5CFA" />
      <path fill="#FFF" d="M8 6h2.2v4.8L14.4 6H17l-4.3 4.9L17.4 18H14.8l-3.5-5.4-1.1 1.2V18H8V6z" />
    </svg>
  );
}
