// Source: https://remocn.dev — copied from the registry; MIT.
export { mixOklch, oklchToRgb, parseColor, rgbToOklch, toCss } from "./color";
export type { EasingName, SpringName } from "./motion";
export { easings, springs } from "./motion";
export type { RemocnTheme, RemocnUIProviderProps } from "./theme";
export { defaultDarkTheme, defaultLightTheme, RemocnUIProvider, useRemocnTheme } from "./theme";
export type { TypewriterOptions, TypewriterState } from "./timeline";
export {
  clamp01,
  framesFor,
  revealCount,
  revealedText,
  useCurrentState,
  useStateTransition,
  useTypewriter,
} from "./timeline";
export type { Step } from "./types";
