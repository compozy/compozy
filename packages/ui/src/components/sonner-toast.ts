import type { ReactElement, ReactNode } from "react";
import { toast as sonnerToast } from "sonner";

export type ToastId = string | number;

export interface ToastAction {
  label: ReactNode;
  onClick: () => void;
}

export interface ToastOptions {
  id?: ToastId;
  description?: ReactNode;
  duration?: number;
  action?: ToastAction;
  /** false keeps the attention overflow toast without a close button. */
  closeButton?: boolean;
  /** false keeps the attention overflow toast from swipe dismissal. */
  dismissible?: boolean;
}

/** A plain object: every method is a property, so tests can `vi.spyOn(toast, "error")`. */
export interface Toast {
  /** The neutral toast, sonner's bare `toast(message)`. */
  message(message: ReactNode, options?: ToastOptions): ToastId;
  success(message: ReactNode, options?: ToastOptions): ToastId;
  error(message: ReactNode, options?: ToastOptions): ToastId;
  info(message: ReactNode, options?: ToastOptions): ToastId;
  warning(message: ReactNode, options?: ToastOptions): ToastId;
  custom(render: (id: ToastId) => ReactNode, options?: ToastOptions): ToastId;
  dismiss(id?: ToastId): void;
  /** Ids of toasts on screen; the attention notifier dismisses its notices when the bell opens. */
  activeIds(): ToastId[];
}

// sonner stores the rendered value as a ReactNode, but its `custom` signature only admits elements.
type SonnerCustomRender = (id: ToastId) => ReactElement;

export const toast: Toast = {
  message: (...args) => sonnerToast(...args),
  success: (...args) => sonnerToast.success(...args),
  error: (...args) => sonnerToast.error(...args),
  info: (...args) => sonnerToast.info(...args),
  warning: (...args) => sonnerToast.warning(...args),
  custom: (render, ...rest) => sonnerToast.custom(render as SonnerCustomRender, ...rest),
  dismiss: (...args) => {
    sonnerToast.dismiss(...args);
  },
  activeIds: () => sonnerToast.getToasts().map(({ id }) => id),
};
