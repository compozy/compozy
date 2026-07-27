import type { SVGProps } from "react";

import { cn } from "../../lib/utils";

export type LogoVariant = "logo" | "symbol" | "lettering";

export interface LogoProps extends Omit<SVGProps<SVGSVGElement>, "children"> {
  variant?: LogoVariant;
  label?: string;
  decorative?: boolean;
}

const LOGO_VIEWBOX: Record<LogoVariant, string> = {
  logo: "0 0 1230 355",
  symbol: "0 0 355 355",
  lettering: "-4 0 808 180",
};

const LOGO_SIZE_CLASS: Record<LogoVariant, string> = {
  logo: "h-8 w-auto",
  symbol: "size-8",
  lettering: "h-8 w-auto",
};

const SYMBOL_PATHS = [
  "M249.48 146.41C232.84 146.41 219.35 159.91 219.35 176.55C219.35 193.19 232.84 206.69 249.48 206.69C266.13 206.69 279.62 193.19 279.62 176.55C279.62 159.91 266.13 146.41 249.48 146.41ZM249.48 190.50C241.78 190.50 235.54 184.25 235.54 176.55C235.54 168.85 241.78 162.60 249.48 162.60C257.18 162.60 263.43 168.85 263.43 176.55C263.43 184.25 257.18 190.50 249.48 190.50Z",
  "M105.28 146.41C88.63 146.41 75.14 159.91 75.14 176.55C75.14 193.19 88.63 206.69 105.28 206.69C121.92 206.69 135.41 193.19 135.41 176.55C135.41 159.91 121.92 146.41 105.28 146.41ZM105.28 190.50C97.57 190.50 91.33 184.25 91.33 176.55C91.33 168.85 97.57 162.60 105.28 162.60C112.98 162.60 119.22 168.85 119.22 176.55C119.22 184.25 112.98 190.50 105.28 190.50Z",
  "M282.15 194.81C282.01 194.67 281.88 194.54 281.74 194.40L280.77 193.45C277.86 198.81 273.61 203.34 268.48 206.59C283.84 223.81 283.27 250.23 266.75 266.75C249.63 283.87 221.88 283.87 204.76 266.75L193.35 255.34L181.78 267.12L196.33 281.40C220.35 304.99 258.95 304.63 282.53 280.61C305.99 256.72 305.77 218.43 282.15 194.81Z",
  "M232.03 145.59L192.82 107.09L180.95 118.96L219.49 157.49C222.62 152.56 226.93 148.47 232.03 145.59V145.59Z",
  "M232.31 207.68C227.28 204.9 222.99 200.93 219.83 196.15L149.23 266.75C132.12 283.87 104.36 283.87 87.24 266.75C70.62 250.13 70.14 223.49 85.79 206.28C81.17 203.24 77.30 199.16 74.52 194.36L72.59 196.33C49.14 220.22 49.36 258.51 72.98 282.14C73.11 282.27 73.25 282.41 73.39 282.54C97.41 306.13 136.01 305.77 159.59 281.75L232.31 207.68L232.31 207.68Z",
  "M85.30 147.15C70.16 129.93 70.80 103.68 87.24 87.24C104.36 70.12 132.12 70.12 149.23 87.24L160.65 98.65L172.21 86.87L157.67 72.59C133.64 49.00 95.04 49.36 71.46 73.38C48.01 97.27 48.23 135.56 71.85 159.18C71.98 159.32 72.12 159.45 72.26 159.59L73.44 160.75C76.17 155.26 80.27 150.57 85.31 147.15H85.30Z",
  "M134.62 196.61C131.32 201.43 126.87 205.39 121.66 208.10L161.17 246.90L173.04 235.03L134.62 196.61Z",
  "M281.40 157.67C304.86 133.78 304.64 95.48 281.02 71.86C280.88 71.72 280.75 71.59 280.61 71.46C256.58 47.87 217.99 48.23 194.40 72.24L122.52 145.47C127.54 148.26 131.82 152.24 134.97 157.02L204.76 87.24C221.88 70.12 249.63 70.12 266.75 87.24C283.09 103.58 283.82 129.60 268.97 146.82C273.61 149.87 277.50 153.98 280.28 158.81L281.41 157.67H281.40Z",
] as const;

const LETTERING_PATHS = [
  "M94 43C81 28 63 21 45 21C21 21 7 45 7 78C7 111 21 135 45 135C63 135 81 128 94 113",
  "M137 54C154 38 180 38 197 54C214 70 214 96 197 112C180 128 154 128 137 112C120 96 120 70 137 54",
  "M245 117V51M245 72C253 53 274 46 288 57C296 63 299 72 299 84V117M299 72C308 53 329 46 343 57C351 63 354 72 354 84V117",
  "M402 154V51M402 67C412 48 437 44 451 59C466 75 466 99 451 113C437 128 412 124 402 105",
  "M500 54C517 38 543 38 560 54C577 70 577 96 560 112C543 128 517 128 500 112C483 96 483 70 500 54",
  "M606 51H677L610 117H681",
  "M728 51L756 117L784 51M756 117L742 151C739 158 733 162 725 162",
] as const;

function SymbolArtwork() {
  return (
    <>
      <rect width="355" height="355" rx="64" fill="#E8572A" />
      {SYMBOL_PATHS.map(d => (
        <path key={d} d={d} fill="#231F20" />
      ))}
    </>
  );
}

function LetteringArtwork() {
  return (
    <>
      {LETTERING_PATHS.map(d => (
        <path
          key={d}
          d={d}
          fill="currentColor"
          fillOpacity="0"
          stroke="currentColor"
          strokeWidth="16"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ))}
    </>
  );
}

function Logo({
  variant = "logo",
  label = "Compozy",
  decorative = false,
  className,
  role,
  "aria-label": ariaLabel,
  ...props
}: LogoProps) {
  return (
    <svg
      {...props}
      data-slot="logo"
      data-variant={variant}
      xmlns="http://www.w3.org/2000/svg"
      viewBox={LOGO_VIEWBOX[variant]}
      fill="none"
      preserveAspectRatio="xMidYMid meet"
      focusable="false"
      role={decorative ? undefined : (role ?? "img")}
      aria-label={decorative ? undefined : (ariaLabel ?? label)}
      aria-hidden={decorative ? true : undefined}
      className={cn("inline-block shrink-0", LOGO_SIZE_CLASS[variant], className)}
    >
      {variant === "logo" && (
        <>
          <g>
            <SymbolArtwork />
          </g>
          <g transform="translate(430 88)">
            <LetteringArtwork />
          </g>
        </>
      )}
      {variant === "symbol" && <SymbolArtwork />}
      {variant === "lettering" && <LetteringArtwork />}
    </svg>
  );
}

export { Logo };
