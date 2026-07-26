import { cn } from "../lib/utils";

export interface BlackboxLogoProps extends React.SVGProps<SVGSVGElement> {}

export function BlackboxLogo({ className, ...props }: BlackboxLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={cn("", className)}
      {...props}
    >
      <title>Blackbox AI</title>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M3 3h18v18H3V3Zm2 2v14h14V5H5Zm3.5 3h4a2.5 2.5 0 0 1 1.66 4.36A2.6 2.6 0 0 1 12.6 16H8.5V8Zm1.6 1.5v2h2.4a1 1 0 0 0 0-2h-2.4Zm0 3.5v2.5h2.5a1.25 1.25 0 0 0 0-2.5h-2.5Z"
      />
    </svg>
  );
}
