import { toast } from "@compozy/ui";

import type { AttentionToastRequest } from "../lib/attention-toast-request";
import { LayoutUpgradeMark } from "./attention-bell-row";
import { AttentionToast, AttentionToastOverflowLedge, ToastFrame } from "./attention-toast";

const ATTENTION_TOAST_DURATION_MS = 8_000;

/**
 * The one entry that turns an attention toast request into a toast. Delivery toasts stay up after a
 * jump; the overflow ledge and the layout-upgrade notice only point at the bell, so activating
 * either dismisses it first.
 */
export function presentAttentionToast(request: AttentionToastRequest): void {
  switch (request.kind) {
    case "delivery":
      toast.custom(
        () => <AttentionToast delivery={request.delivery} onActivate={request.onActivate} />,
        { id: request.delivery.key, duration: ATTENTION_TOAST_DURATION_MS }
      );
      return;
    case "overflow":
      toast.custom(
        id => (
          <AttentionToastOverflowLedge
            count={request.count}
            onActivate={() => {
              toast.dismiss(id);
              request.onActivate();
            }}
          />
        ),
        {
          id: request.id,
          duration: ATTENTION_TOAST_DURATION_MS,
          closeButton: false,
          dismissible: false,
        }
      );
      return;
    case "layout-upgrade":
      toast.custom(
        id => (
          <ToastFrame
            testId="os-attention-toast-layout-upgrade"
            mark={<LayoutUpgradeMark />}
            title={request.title}
            body={request.body}
            meta={request.meta}
            onActivate={() => {
              toast.dismiss(id);
              request.onActivate();
            }}
          />
        ),
        { id: request.id, duration: ATTENTION_TOAST_DURATION_MS }
      );
  }
}
