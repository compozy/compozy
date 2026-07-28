import type { SVGProps } from "react";

export function TelegramLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      className={className || "size-8"}
    >
      <title>Telegram Logo</title>
      <circle cx="12" cy="12" r="12" fill="#26A5E4" />
      <path
        fill="#fff"
        d="M5.46 11.77s5.77-2.37 7.77-3.2c.77-.34 3.37-1.4 3.37-1.4s1.2-.47 1.1.67c-.03.47-.3 2.13-.57 3.93-.4 2.54-.83 5.3-.83 5.3s-.07.77-.64.9c-.57.13-1.5-.47-1.67-.6-.13-.1-2.53-1.63-3.4-2.37-.23-.2-.5-.6.03-1.07 1.2-1.1 2.63-2.46 3.5-3.33.4-.4.8-1.33-.87-.2-2.37 1.63-4.7 3.17-4.7 3.17s-.53.33-1.53.03c-1-.3-2.17-.7-2.17-.7s-.8-.5.6-1.03z"
      />
    </svg>
  );
}
