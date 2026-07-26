import { cn } from "../lib/utils";

export interface HermesLogoProps extends React.SVGProps<SVGSVGElement> {}

export function HermesLogo({ className, ...props }: HermesLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={cn("", className)}
      {...props}
    >
      <title>Hermes</title>
      <path d="M3.5 6.5 12 3l8.5 3.5-1.4 1.6L12 5.2 4.9 8.1 3.5 6.5Z" />
      <path d="M6 9.5h2v11H6v-11Zm10 0h2v11h-2v-11Zm-7 0h6v4.2H9V9.5Zm0 5.7h6v5.3H9v-5.3Z" />
    </svg>
  );
}
