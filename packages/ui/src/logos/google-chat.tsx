import type { SVGProps } from "react";

export function GoogleChatLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      className={className || "size-8"}
    >
      <title>Google Chat Logo</title>
      <path
        fill="#00AC47"
        d="M20.182 3H3.818A1.818 1.818 0 0 0 2 4.818v13.091c0 1.004.814 1.818 1.818 1.818h1.818v3.137c0 .497.59.746.935.397l3.535-3.534h10.076c1.003 0 1.818-.814 1.818-1.818V4.818A1.818 1.818 0 0 0 20.182 3z"
      />
      <path fill="#FFF" d="M17 9H7V7.5h10V9zm0 3H7v-1.5h10V12zm-4 3H7v-1.5h6V15z" />
    </svg>
  );
}
