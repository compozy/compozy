import { cn } from "../lib/utils";

export interface MistralLogoProps extends React.SVGProps<SVGSVGElement> {}

export function MistralLogo({ className, ...props }: MistralLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      preserveAspectRatio="xMidYMid"
      viewBox="0 0 256 233"
      className={cn("", className)}
      {...props}
    >
      <title>Mistral</title>
      <path d="M186.18 0h46.54v46.54h-46.54z" />
      <path fill="#F7D046" d="M209.45 0h46.54v46.54h-46.54z" />
      <path d="M0 0h46.54v46.54H0zM0 46.54h46.54V93.09H0zM0 93.09h46.54v46.54H0zM0 139.63h46.54v46.54H0zM0 186.18h46.54v46.54H0z" />
      <path fill="#F7D046" d="M23.27 0h46.54v46.54H23.27z" />
      <path fill="#F2A73B" d="M209.45 46.54h46.54V93.09h-46.54zM23.27 46.54h46.54V93.09H23.27z" />
      <path d="M139.63 46.54h46.54V93.09h-46.54z" />
      <path fill="#F2A73B" d="M162.90 46.54h46.54V93.09h-46.54zM69.81 46.54h46.54V93.09H69.81z" />
      <path
        fill="#EE792F"
        d="M116.36 93.09h46.54v46.54h-46.54zM162.90 93.09h46.54v46.54h-46.54zM69.81 93.09h46.54v46.54H69.81z"
      />
      <path d="M93.09 139.63h46.54v46.54H93.09z" />
      <path fill="#EB5829" d="M116.36 139.63h46.54v46.54h-46.54z" />
      <path fill="#EE792F" d="M209.45 93.09h46.54v46.54h-46.54zM23.27 93.09h46.54v46.54H23.27z" />
      <path d="M186.18 139.63h46.54v46.54h-46.54z" />
      <path fill="#EB5829" d="M209.45 139.63h46.54v46.54h-46.54z" />
      <path d="M186.18 186.18h46.54v46.54h-46.54z" />
      <path fill="#EB5829" d="M23.27 139.63h46.54v46.54H23.27z" />
      <path fill="#EA3326" d="M209.45 186.18h46.54v46.54h-46.54zM23.27 186.18h46.54v46.54H23.27z" />
    </svg>
  );
}
