import { cn } from "../lib/utils";

export interface JunieLogoProps extends React.SVGProps<SVGSVGElement> {}

export function JunieLogo({ className, ...props }: JunieLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 48 48"
      fill="none"
      className={cn("", className)}
      {...props}
    >
      <title>Junie</title>
      <path
        fill="#48E054"
        d="M31.993 16.013H48v2.667C48 37.346 39.997 48 18.668 48H16V32h2.668c9.33 0 13.338-3.993 13.338-13.333V16zM16 16H0v16h16zM32 0H16v16h16z"
      />
    </svg>
  );
}
