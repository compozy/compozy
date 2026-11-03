import { useNavigate } from "@tanstack/react-router";
import { History } from "@compozy/ui/icons";

import { Marker } from "@compozy/ui";

/** Where a rotated conversation came from, with the way back to it. */
export function RotatedMarker({ predecessorId }: { predecessorId: string | undefined }) {
  const navigate = useNavigate();
  return (
    <Marker icon={<History aria-hidden="true" className="size-3" />}>
      This conversation continues from an earlier one.{" "}
      {predecessorId ? (
        <a
          className="underline"
          href={`/session/${encodeURIComponent(predecessorId)}`}
          onClick={event => {
            event.preventDefault();
            void navigate({ to: "/session/$id", params: { id: predecessorId } });
          }}
        >
          Read it
        </a>
      ) : null}
    </Marker>
  );
}
