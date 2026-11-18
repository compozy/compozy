// One product icon as inline SVG. size in px; stroke stays 1.5 at 24 and scales with the icon.
import { icons, type IconName } from "./icons";

interface IconProps {
  name: IconName;
  size?: number;
  className?: string;
  stroke?: number;
  title?: string;
}

export function Icon({ name, size = 24, className, stroke = 1.5, title }: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={stroke}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      role={title ? "img" : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
      // Paths are static markup from the icon set data (icons.ts), never user input.
      dangerouslySetInnerHTML={{ __html: icons[name].path }}
    />
  );
}
