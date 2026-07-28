import { cn } from "../lib/utils";

export interface ZAILogoProps extends React.SVGProps<SVGSVGElement> {}

export function ZAILogo({ className, ...props }: ZAILogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 30 30"
      fill="currentColor"
      className={cn("", className)}
      {...props}
    >
      <title>Z.AI</title>
      <path d="M15.47 7.1l-1.3 1.85c-.2.29-.54.47-.9.47h-7.1V7.09H15.47Z" />
      <path d="M24.3 7.1L13.14 22.91H5.7L16.86 7.1H24.3Z" />
      <path d="M14.53 22.91l1.31-1.86c.2-.29.54-.47.9-.47h7.09v2.33H14.53Z" />
    </svg>
  );
}
