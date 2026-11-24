import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Info } from "@compozy/ui/icons";
import { Alert, AlertDescription, AlertTitle, Button, buttonVariants, toast } from "@compozy/ui";

import { dismissMigrationNotice } from "../adapters/migration-notices-api";
import { migrationNoticesOptions } from "../lib/query-options";

const launchPost = "https://compozy.com/blog/compozy-0-8-remote-computers";

export function GatewayUpgradeNotice() {
  const queryClient = useQueryClient();
  const options = migrationNoticesOptions();
  const { data } = useQuery(options);
  const notice = data?.notices.find(item => item.key === "gateway_removed");
  const dismiss = useMutation({
    mutationFn: dismissMigrationNotice,
    onSuccess: (_result, key) => {
      queryClient.setQueryData(options.queryKey, current =>
        current ? { notices: current.notices.filter(item => item.key !== key) } : current
      );
      return queryClient.invalidateQueries({ queryKey: options.queryKey });
    },
    onError: error => toast.error(error.message),
  });

  if (!notice) return null;
  const count = notice.payload.bindings_needing_confirmation;
  return (
    <Alert className="gap-y-1 py-2.5" variant="info">
      <Info aria-hidden="true" />
      <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
        <span>Remote access was replaced by Computers</span>
        <div className="flex shrink-0 items-center gap-1.5">
          {count > 0 ? (
            <Link
              className={buttonVariants({ size: "xs", variant: "secondary" })}
              search={{ start: "event" }}
              to="/automations"
            >
              Review
            </Link>
          ) : null}
          <a
            className={buttonVariants({ size: "xs", variant: "ghost" })}
            href={launchPost}
            rel="noreferrer"
            target="_blank"
          >
            Remote Computers launch notes
          </a>
          <Button
            disabled={dismiss.isPending}
            onClick={() => dismiss.mutate(notice.key)}
            size="xs"
            type="button"
            variant="ghost"
          >
            Dismiss
          </Button>
        </div>
      </AlertTitle>
      <AlertDescription>
        <p>
          Paired devices and connection profiles were removed. Set up remote computers again.
          {count > 0 ? (
            <>
              {" "}
              <span className="font-medium text-fg">
                {count === 1 ? "1 webhook trigger needs" : count + " webhook triggers need"}{" "}
                confirmation for {count === 1 ? "its" : "their"} new address.
              </span>
            </>
          ) : null}
        </p>
      </AlertDescription>
    </Alert>
  );
}
