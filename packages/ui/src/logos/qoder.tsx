import { cn } from "../lib/utils";

export interface QoderLogoProps extends React.SVGProps<SVGSVGElement> {}

export function QoderLogo({ className, ...props }: QoderLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={cn("", className)}
      {...props}
    >
      <title>Qoder</title>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12 3a9 9 0 1 0 6.34 15.4l2.04 2.04 1.41-1.41-2.04-2.04A9 9 0 0 0 12 3Zm0 2a7 7 0 1 1 0 14 7 7 0 0 1 0-14Zm0 2.5a4.5 4.5 0 1 0 0 9 4.5 4.5 0 0 0 0-9Z"
      />
    </svg>
  );
}
