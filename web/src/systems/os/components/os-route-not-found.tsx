import { useEffect } from "react";
import { toast } from "sonner";

/**
 * Not-found posture on the desktop: the shell stays up (windows untouched);
 * the unknown path is reported once, non-blockingly.
 */
export function OsRouteNotFound() {
  useEffect(() => {
    toast("Page not found", {
      description: "That link doesn't open anything.",
    });
  }, []);
  return null;
}
