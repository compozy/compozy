import { forwardRef } from "react";
import { cn } from "@/lib/utils";

export interface IconProps extends React.SVGAttributes<SVGElement> {
  name: string;
  size?: number;
  strokeWidth?: number;
}

const Icon = forwardRef<SVGSVGElement, IconProps>(
  ({ name, size = 24, strokeWidth = 2, className, ...props }, ref) => {
    // Transform icon name to PascalCase for lucide-static compatibility
    const transformedName = name
      // Handle kebab-case to PascalCase
      .replace(/-([a-z])/g, (_, letter) => letter.toUpperCase())
      // Ensure first letter is uppercase
      .replace(/^[a-z]/, (letter) => letter.toUpperCase());

    try {
      // Import the icon from lucide-static using Node.js approach
      const lucideStatic = require('lucide-static');
      const svgString = lucideStatic[transformedName];
      
      if (!svgString) {
        throw new Error(`Icon "${name}" (${transformedName}) not found`);
      }

      // For SSR compatibility, use dangerouslySetInnerHTML with the SVG content
      // Extract the inner content from the SVG
      const svgMatch = svgString.match(/<svg[^>]*>(.*?)<\/svg>/s);
      if (!svgMatch) {
        throw new Error(`Invalid SVG format for icon "${name}"`);
      }
      
      const svgContent = svgMatch[1];

      return (
        <svg
          ref={ref}
          width={size}
          height={size}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
          className={cn("lucide", className)}
          {...props}
          dangerouslySetInnerHTML={{ __html: svgContent }}
        />
      );
    } catch (error) {
      console.warn(`Icon "${name}" (${transformedName}) not found in lucide-static:`, error);
      
      // Fallback to a basic question mark icon
      return (
        <svg
          ref={ref}
          width={size}
          height={size}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
          className={cn("lucide", className)}
          {...props}
        >
          <circle cx="12" cy="12" r="10" />
          <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
          <path d="M12 17h.01" />
        </svg>
      );
    }
  }
);

Icon.displayName = "Icon";

export { Icon };