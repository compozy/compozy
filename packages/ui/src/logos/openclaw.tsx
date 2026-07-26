import { cn } from "../lib/utils";

export interface OpenClawLogoProps extends React.SVGProps<SVGSVGElement> {}

export function OpenClawLogo({ className, ...props }: OpenClawLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 16 16"
      role="img"
      aria-label="OpenClaw"
      className={cn("", className)}
      {...props}
    >
      <title>OpenClaw</title>
      <g fill="#3A0A0D">
        <path d="M1 5h1v3H1zM2 4h1v1H2zM2 8h1v1H2zM3 3h1v1H3zM3 9h1v1H3zM4 2h1v1H4zM4 10h1v1H4zM5 2h6v1H5zM11 2h1v1h-1zM12 3h1v1h-1zM12 9h1v1h-1zM13 4h1v1h-1zM13 8h1v1h-1zM14 5h1v3h-1zM5 11h6v1H5zM4 12h1v1H4zM11 12h1v1h-1zM3 13h1v1H3zM12 13h1v1h-1zM5 14h6v1H5z" />
      </g>
      <g fill="#FF4F40">
        <path d="M5 3h6v1H5zM4 4h8v1H4zM3 5h10v1H3zM3 6h10v1H3zM3 7h10v1H3zM4 8h8v1H4zM5 9h6v1H5zM5 12h6v1H5zM6 13h4v1H6z" />
      </g>
      <g fill="#FF775F">
        <path d="M1 6h2v1H1zM2 5h1v1H2zM2 7h1v1H2zM13 6h2v1h-2zM13 5h1v1h-1zM13 7h1v1h-1z" />
      </g>
      <g fill="#081016">
        <path d="M6 5h1v1H6zM9 5h1v1H9z" />
      </g>
      <g fill="#F5FBFF">
        <path d="M6 4h1v1H6zM9 4h1v1H9z" />
      </g>
    </svg>
  );
}
