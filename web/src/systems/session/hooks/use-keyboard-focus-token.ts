import { useRef, useState } from "react";

/** Each keyboard focus gets its own id; pointer focus reveals nothing. */
export function useKeyboardFocusToken() {
  const focusCount = useRef(0);
  const [keyboardFocus, setKeyboardFocus] = useState<number | null>(null);
  const focusHandlers = {
    onFocus: (event: React.FocusEvent<HTMLElement>) => {
      focusCount.current += 1;
      setKeyboardFocus(event.currentTarget.matches(":focus-visible") ? focusCount.current : null);
    },
    onBlur: () => setKeyboardFocus(null),
  };
  return { keyboardFocus, focusHandlers };
}
