import { useEffect } from "react";
import { toast } from "sonner";

/**
 * Not-found posture on the desktop: the shell stays up (windows untouched);
 * the unknown path is reported once, non-blockingly.
 */
export function OsRouteNotFound() {
  useEffect(() => {
    toast("Nothing lives at this address", {
      description: "No app owns this path. The desktop is unchanged.",
    });
  }, []);
  return null;
}
