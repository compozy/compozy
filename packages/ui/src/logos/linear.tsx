import { useId, type SVGProps } from "react";

interface LinearLogoProps extends SVGProps<SVGSVGElement> {
  variant?: "icon" | "logo" | "wordmark";
  mode?: "dark" | "light";
}

const COLORS = {
  dark: "#fff", // Use light color on dark backgrounds
  light: "#222326", // Use dark color on light backgrounds
};

/**
 * Linear Logo Component
 *
 * Brand Guidelines:
 * - Wordmark: Primary logo, use when space allows
 * - Logo: Logomark for tight layouts or logo-only grids (default)
 * - Icon: Stylized app icon for social media or chip designs
 *
 * Usage:
 * - Default is logo variant (logomark)
 * - Provide plenty of space around logo assets
 * - Monochrome usage preferred with brand colors
 */
export function LinearLogo({
  className,
  variant = "logo",
  mode = "dark",
  ...props
}: LinearLogoProps) {
  const color = COLORS[mode];
  const idPrefix = `linear-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const iconId = (suffix: string) => `${idPrefix}-${suffix}`;

  if (variant === "icon") {
    return (
      <svg
        {...props}
        className={className}
        xmlns="http://www.w3.org/2000/svg"
        width="256"
        height="256"
        fill="none"
        viewBox="0 0 512 512"
      >
        <title>Linear Icon</title>
        <path fill={`url(#${iconId("a")})`} d="M0 0h512v512H0z" />
        <g filter={`url(#${iconId("b")})`} opacity=".8">
          <path
            fill="#fff"
            d="M346.11 342.26c1.67 1.67 4.36 1.77 6.11.168 58.50-53.76 61.2-148.75 4.50-205.44-56.69-56.69-151.68-53.99-205.44 4.50-1.60 1.74-1.50 4.43.168 6.11l194.66 194.66Z"
            shapeRendering="crispEdges"
            style={{ mixBlendMode: "multiply" }}
          />
        </g>
        <g filter={`url(#${iconId("c")})`} opacity=".3">
          <path
            fill={`url(#${iconId("d")})`}
            d="M346.11 342.26c1.67 1.67 4.36 1.77 6.11.168 58.50-53.76 61.2-148.75 4.50-205.44-56.69-56.69-151.68-53.99-205.44 4.50-1.60 1.74-1.50 4.43.168 6.11l194.66 194.66Z"
          />
        </g>
        <g filter={`url(#${iconId("e")})`} opacity=".3">
          <path
            fill={`url(#${iconId("f")})`}
            d="M261.60 324.79c2.44-1.43 2.84-4.78.912-6.85L126.12 171.95c-2.01-2.16-5.53-1.83-7.01.727a148.99 148.99 0 0 0-6.49 12.53c-.774 1.68-.389 3.67.926 4.98l137.08 136.59a4.51 4.51 0 0 0 4.98.944c2.70-1.17 4.02-1.79 5.99-2.94Z"
          />
        </g>
        <path
          fill={`url(#${iconId("g")})`}
          d="M357.35 374.30c1.75 1.75 4.58 1.86 6.41.189a163.59 163.59 0 0 0 5.31-5.08c62.54-62.54 62.54-163.95 0-226.50-62.54-62.54-163.95-62.54-226.50 0a163.59 163.59 0 0 0-5.08 5.31c-1.67 1.83-1.56 4.65.189 6.41l219.66 219.66Z"
        />
        <path
          fill={`url(#${iconId("h")})`}
          d="M357.35 374.30c1.75 1.75 4.58 1.86 6.41.189a163.59 163.59 0 0 0 5.31-5.08c62.54-62.54 62.54-163.95 0-226.50-62.54-62.54-163.95-62.54-226.50 0a163.59 163.59 0 0 0-5.08 5.31c-1.67 1.83-1.56 4.65.189 6.41l219.66 219.66Z"
        />
        <path
          fill={`url(#${iconId("i")})`}
          d="M336.33 394.67c2.62-1.52 3.02-5.11.875-7.26L124.59 174.79c-2.14-2.14-5.73-1.75-7.26.875a158.87 158.87 0 0 0-7.11 13.72c-.811 1.77-.41 3.85.968 5.22l206.20 206.20c1.37 1.37 3.45 1.77 5.23.96a158.87 158.87 0 0 0 13.72-7.11Z"
        />
        <path
          fill={`url(#${iconId("j")})`}
          d="M336.33 394.67c2.62-1.52 3.02-5.11.875-7.26L124.59 174.79c-2.14-2.14-5.73-1.75-7.26.875a158.87 158.87 0 0 0-7.11 13.72c-.811 1.77-.41 3.85.968 5.22l206.20 206.20c1.37 1.37 3.45 1.77 5.23.96a158.87 158.87 0 0 0 13.72-7.11Z"
        />
        <path
          fill={`url(#${iconId("k")})`}
          d="M286.65 413.34c3.61-.707 4.86-5.13 2.25-7.74L106.39 223.08c-2.60-2.60-7.03-1.36-7.74 2.25a160.81 160.81 0 0 0-2.50 18.46 4.66 4.66 0 0 0 1.36 3.65l167.02 167.02a4.66 4.66 0 0 0 3.65 1.36 160.83 160.83 0 0 0 18.46-2.50Z"
        />
        <path
          fill={`url(#${iconId("l")})`}
          d="M286.65 413.34c3.61-.707 4.86-5.13 2.25-7.74L106.39 223.08c-2.60-2.60-7.03-1.36-7.74 2.25a160.81 160.81 0 0 0-2.50 18.46 4.66 4.66 0 0 0 1.36 3.65l167.02 167.02a4.66 4.66 0 0 0 3.65 1.36 160.83 160.83 0 0 0 18.46-2.50Z"
        />
        <path
          fill={`url(#${iconId("m")})`}
          d="M217.03 411.57c4.45 1.10 7.20-4.15 3.95-7.39L107.82 291.01c-3.24-3.24-8.50-.491-7.39 3.95 6.78 27.27 20.83 53.12 42.16 74.44 21.32 21.32 47.16 35.37 74.44 42.16Z"
        />
        <path
          fill={`url(#${iconId("n")})`}
          d="M217.03 411.57c4.45 1.10 7.20-4.15 3.95-7.39L107.82 291.01c-3.24-3.24-8.50-.491-7.39 3.95 6.78 27.27 20.83 53.12 42.16 74.44 21.32 21.32 47.16 35.37 74.44 42.16Z"
        />
        <path
          stroke="#fff"
          strokeOpacity=".5"
          strokeWidth="5"
          d="M362.08 372.64c-.816.74-2.11.733-2.96-.111L139.46 152.87c-.844-.844-.857-2.14-.111-2.96a160.66 160.66 0 0 1 5.00-5.23c61.57-61.57 161.39-61.57 222.96 0 61.57 61.57 61.57 161.39 0 222.96a160.66 160.66 0 0 1-5.23 5.00Zm-26.64 16.52c1.03 1.03.786 2.67-.364 3.34a156.56 156.56 0 0 1-13.51 7.00c-.794.36-1.76.197-2.42-.462L112.94 192.85c-.659-.659-.826-1.62-.462-2.42a156.56 156.56 0 0 1 7.00-13.51c.67-1.15 2.30-1.40 3.34-.364L335.44 389.17Zm-48.29 18.20c1.27 1.27.574 3.22-.964 3.52a158.26 158.26 0 0 1-18.17 2.46 2.16 2.16 0 0 1-1.69-.64L99.28 245.68a2.16 2.16 0 0 1-.64-1.69 158.31 158.31 0 0 1 2.46-18.17c.3-1.53 2.24-2.24 3.52-.964l182.51 182.51Zm-67.92-1.42c.81.81.81 1.73.464 2.39-.333.63-1.00 1.07-2.05.813-26.85-6.67-52.28-20.51-73.28-41.50-20.99-20.99-34.82-46.43-41.50-73.28-.259-1.04.183-1.71.813-2.05.656-.348 1.58-.346 2.39.464l113.16 113.16Z"
          style={{ mixBlendMode: "soft-light" }}
        />
        <defs>
          <linearGradient
            id={iconId("a")}
            x1="256"
            x2="256"
            y1="0"
            y2="512"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#2D2E31" />
            <stop offset="1" stopColor="#0F1012" />
          </linearGradient>
          <linearGradient
            id={iconId("d")}
            x1="256.30"
            x2="256.30"
            y1="95.33"
            y2="379.49"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#C5C5C5" />
          </linearGradient>
          <linearGradient
            id={iconId("f")}
            x1="178.36"
            x2="178.36"
            y1="167.24"
            y2="351.12"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#C5C5C5" />
          </linearGradient>
          <linearGradient
            id={iconId("g")}
            x1="256"
            x2="256"
            y1="96"
            y2="416"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#CCC" />
          </linearGradient>
          <linearGradient
            id={iconId("i")}
            x1="256"
            x2="256"
            y1="96"
            y2="416"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#CCC" />
          </linearGradient>
          <linearGradient
            id={iconId("k")}
            x1="256"
            x2="256"
            y1="96"
            y2="416"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#CCC" />
          </linearGradient>
          <linearGradient
            id={iconId("m")}
            x1="256"
            x2="256"
            y1="96"
            y2="416"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset="1" stopColor="#CCC" />
          </linearGradient>
          <radialGradient
            id={iconId("h")}
            cx="0"
            cy="0"
            r="1"
            gradientTransform="matrix(0 320 -320 0 256 96)"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset=".598" stopColor="#fff" stopOpacity="0" />
          </radialGradient>
          <radialGradient
            id={iconId("j")}
            cx="0"
            cy="0"
            r="1"
            gradientTransform="matrix(0 320 -320 0 256 96)"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset=".598" stopColor="#fff" stopOpacity="0" />
          </radialGradient>
          <radialGradient
            id={iconId("l")}
            cx="0"
            cy="0"
            r="1"
            gradientTransform="matrix(0 320 -320 0 256 96)"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset=".598" stopColor="#fff" stopOpacity="0" />
          </radialGradient>
          <radialGradient
            id={iconId("n")}
            cx="0"
            cy="0"
            r="1"
            gradientTransform="matrix(0 320 -320 0 256 96)"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#fff" />
            <stop offset=".598" stopColor="#fff" stopOpacity="0" />
          </radialGradient>
          <filter
            id={iconId("b")}
            width="295.58"
            height="295.58"
            x="126.13"
            y="66.88"
            colorInterpolationFilters="sRGB"
            filterUnits="userSpaceOnUse"
          >
            <feFlood floodOpacity="0" result="BackgroundImageFix" />
            <feColorMatrix
              in="SourceAlpha"
              result="hardAlpha"
              values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
            />
            <feOffset dy="-5.12" />
            <feGaussianBlur stdDeviation="12" />
            <feComposite in2="hardAlpha" operator="out" />
            <feColorMatrix values="0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 0 0.4 0" />
            <feBlend
              in2="BackgroundImageFix"
              mode="plus-lighter"
              result="effect1_dropShadow_14134_4654"
            />
            <feBlend in="SourceGraphic" in2="effect1_dropShadow_14134_4654" result="shape" />
          </filter>
          <filter
            id={iconId("c")}
            width="267.58"
            height="267.58"
            x="140.13"
            y="86"
            colorInterpolationFilters="sRGB"
            filterUnits="userSpaceOnUse"
          >
            <feFlood floodOpacity="0" result="BackgroundImageFix" />
            <feBlend in="SourceGraphic" in2="BackgroundImageFix" result="shape" />
            <feGaussianBlur result="effect1_foregroundBlur_14134_4654" stdDeviation="5" />
          </filter>
          <filter
            id={iconId("e")}
            width="171.52"
            height="177.59"
            x="102.21"
            y="160.52"
            colorInterpolationFilters="sRGB"
            filterUnits="userSpaceOnUse"
          >
            <feFlood floodOpacity="0" result="BackgroundImageFix" />
            <feBlend in="SourceGraphic" in2="BackgroundImageFix" result="shape" />
            <feGaussianBlur result="effect1_foregroundBlur_14134_4654" stdDeviation="5" />
          </filter>
        </defs>
      </svg>
    );
  }

  if (variant === "logo") {
    return (
      <svg
        {...props}
        className={className}
        xmlns="http://www.w3.org/2000/svg"
        fill={color}
        width="200"
        height="200"
        viewBox="0 0 100 100"
      >
        <title>Linear Logo</title>
        <path d="M1.22 61.52c-.22-.95.9-1.54 1.59-.86l36.52 36.51c.69.68.09 1.81-.86 1.59-18.42-4.31-32.93-18.82-37.25-37.24ZM0 46.88c-.02.28.09.55.29.76l52.06 52.06c.2.2.48.3.76.28 2.36-.15 4.69-.46 6.96-.93.76-.16 1.03-1.09.48-1.64L2.57 39.44c-.55-.55-1.49-.29-1.64.48C.46 42.18.15 44.51 0 46.88ZM4.21 29.7c-.17.37-.08.81.21 1.1l64.77 64.77c.29.28.73.37 1.1.2 1.78-.8 3.51-1.69 5.18-2.68.55-.33.63-1.08.18-1.54L8.43 24.33c-.45-.45-1.21-.37-1.54.18-.99 1.66-1.88 3.39-2.68 5.18ZM12.65 18.07c-.37-.37-.39-.96-.04-1.35C21.77 6.45 35.11 0 49.95 0 77.59 0 100 22.4 100 50.04c0 14.84-6.45 28.17-16.71 37.33-.39.34-.98.32-1.35-.04L12.65 18.07Z" />
      </svg>
    );
  }

  // Default: wordmark
  return (
    <svg
      {...props}
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="400"
      height="100"
      viewBox="0 0 400 100"
      fill={color}
    >
      <title>Linear Wordmark</title>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12.92 16.37c-.53.58-.49 1.47.06 2.02l68.59 68.59c.56.55 1.44.59 2.02.06 10.05-9.15 16.37-22.34 16.37-37.01C99.98 22.4 77.57 0 49.94 0 35.27 0 22.07 6.31 12.92 16.37ZM4.35 29.38c-.25.55-.13 1.21.31 1.64l64.28 64.29c.43.43 1.08.56 1.64.31 1.48-.67 2.93-1.41 4.33-2.22.83-.48.96-1.61.28-2.3L8.88 24.77c-.68-.68-1.81-.55-2.3.28-.81 1.4-1.55 2.84-2.22 4.33ZM.45 47.79c-.3-.3-.46-.72-.43-1.14.13-1.98.38-3.94.74-5.86.21-1.15 1.62-1.54 2.44-.72l56.71 56.7c.83.82.43 2.23-.72 2.44-1.91.36-3.87.61-5.86.74-.42.02-.84-.13-1.14-.43L.45 47.79ZM3.93 61.75c-1.03-1.03-2.7-.14-2.32 1.26 4.61 17.21 18.15 30.74 35.34 35.35 1.41.38 2.3-1.28 1.26-2.32L3.93 61.75ZM201.6 27.53c3.58 0 6.49-2.91 6.49-6.51s-2.91-6.52-6.49-6.52-6.49 2.91-6.49 6.51 2.9 6.51 6.49 6.51Zm-55.62 56.83V14.5h11.54v59.64h31.11v10.22h-42.65Zm82.13-28.51v28.51h-11.16V34.85h11.02v8.48l.14-.09c1.12-2.65 2.92-4.87 5.42-6.65 2.49-1.81 5.66-2.71 9.53-2.71 3.42 0 6.54.76 9.34 2.29 2.8 1.5 5.04 3.7 6.72 6.61 1.68 2.9 2.52 6.47 2.52 10.69v30.9h-11.16V55.01c0-3.75-1-6.59-2.99-8.53-1.96-1.96-4.59-2.95-7.89-2.95-2.11 0-4.04.44-5.79 1.31-1.74.88-3.13 2.21-4.15 4.03-1.02 1.81-1.54 4.14-1.54 6.98Zm101.1 27.66c2.55 1.09 5.48 1.64 8.78 1.64 2.71 0 5.03-.34 6.96-1.03 1.93-.72 3.52-1.67 4.76-2.86 1.27-1.18 2.28-2.48 3.03-3.89h.19v6.98h10.69V50.28c0-2.4-.47-4.61-1.4-6.61-.93-2-2.28-3.73-4.06-5.2-1.74-1.46-3.86-2.59-6.35-3.37-2.49-.81-5.29-1.21-8.4-1.21-4.26 0-7.95.73-11.07 2.2-3.08 1.43-5.49 3.37-7.24 5.81-1.74 2.43-2.69 5.18-2.85 8.25h10.79c.12-1.43.62-2.71 1.49-3.84.87-1.12 2.05-2 3.55-2.62 1.49-.66 3.22-.98 5.18-.98s3.62.33 4.99.98c1.4.66 2.47 1.54 3.22 2.67.75 1.12 1.12 2.43 1.12 3.93v.38c0 1.12-.39 1.95-1.16 2.48-.75.53-2.02.92-3.83 1.17-1.77.25-4.2.55-7.28.89-2.52.28-4.95.7-7.28 1.26-2.33.56-4.42 1.39-6.26 2.48-1.8 1.09-3.23 2.54-4.29 4.36-1.05 1.81-1.58 4.14-1.58 6.98 0 3.28.75 6.03 2.24 8.25 1.49 2.18 3.52 3.84 6.07 4.97Zm18.08-8.3c-1.8.97-4.03 1.45-6.68 1.45-2.67 0-4.81-.56-6.4-1.68-1.58-1.15-2.38-2.73-2.38-4.73 0-1.56.44-2.82 1.3-3.79.9-.97 2.08-1.73 3.55-2.29 1.46-.56 3.05-.95 4.76-1.17 1.24-.19 2.46-.38 3.64-.56 1.18-.22 2.28-.42 3.31-.61 1.02-.22 1.9-.44 2.61-.66.74-.22 1.29-.45 1.63-.7V66c0 1.93-.45 3.72-1.35 5.34-.87 1.59-2.21 2.89-4.01 3.89Zm26.09 9.14v-49.5h10.74V43h.14c.9-2.81 2.32-4.95 4.25-6.42 1.96-1.5 4.53-2.25 7.7-2.25.78 0 1.48.03 2.1.09.65.03 1.2.06 1.63.09v10.08c-.41-.06-1.12-.14-2.14-.23-1.02-.09-2.11-.14-3.27-.14-1.83 0-3.51.42-5.04 1.26-1.52.84-2.74 2.14-3.64 3.89-.87 1.71-1.3 3.89-1.3 6.51v28.46h-11.16Zm-177.4 0v-49.5h11.16v49.51h-11.16Zm84.23-2.2c3.58 2.21 7.83 3.32 12.75 3.32 3.8 0 7.25-.69 10.37-2.06 3.14-1.4 5.76-3.32 7.84-5.76 2.08-2.46 3.44-5.31 4.06-8.53h-10.51c-.47 1.46-1.23 2.76-2.29 3.89-1.02 1.09-2.32 1.95-3.87 2.57-1.55.63-3.36.94-5.42.93-2.77 0-5.15-.63-7.14-1.87-1.96-1.25-3.45-2.98-4.48-5.2-.93-2.04-1.44-4.35-1.52-6.94h35.91v-3c0-3.81-.56-7.28-1.68-10.41-1.12-3.15-2.71-5.87-4.76-8.15-2.05-2.31-4.53-4.09-7.42-5.34-2.86-1.25-6.05-1.87-9.57-1.87-4.57 0-8.62 1.1-12.14 3.32-3.52 2.21-6.27 5.28-8.27 9.19-1.99 3.9-2.99 8.37-2.99 13.41 0 5 .97 9.45 2.89 13.36 1.93 3.87 4.68 6.92 8.26 9.14Zm23.5-32.77c-1.02-2.12-2.49-3.76-4.39-4.92-1.9-1.15-4.14-1.73-6.72-1.73-2.55 0-4.78.58-6.68 1.73-1.86 1.15-3.33 2.79-4.39 4.92-.76 1.53-1.23 3.29-1.43 5.25h25.05c-.2-1.96-.68-3.71-1.43-5.25Z"
      />
    </svg>
  );
}
