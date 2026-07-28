import type { SVGProps } from "react";

export function MicrosoftTeamsLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      className={className || "size-8"}
    >
      <title>Microsoft Teams Logo</title>
      <path
        fill="#5059C9"
        d="M14.5 9h4.9c.5 0 .9.4.9.9v4.4c0 1.8-1.5 3.3-3.3 3.3-1.8 0-3.3-1.5-3.3-3.3V9.5c0-.3.3-.5.8-.5zm4.1-1.5c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2z"
      />
      <path
        fill="#7B83EB"
        d="M11 8c1.7 0 3 1.3 3 3s-1.3 3-3 3-3-1.3-3-3 1.3-3 3-3zM3 7h12c.6 0 1 .4 1 1v10.5c0 1.9-1.6 3.5-3.5 3.5h-7C3.6 22 2 20.4 2 18.5V8c0-.6.4-1 1-1z"
      />
      <path fill="#FFF" d="M9 10H5v1.5h1.5V16H8v-4.5h1zm-2 8.5c2 0 3.5-.4 3.5-.4v-9H7v9.4z" />
    </svg>
  );
}
