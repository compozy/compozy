import type { AttentionDelivery } from "./attention-delivery";

/**
 * What an attention toast says, as plain data. The notifier hooks decide which toasts to raise and
 * what activating one does; the components layer (`presentAttentionToast`) owns how each looks,
 * how long it stays and whether activating it dismisses it.
 */
export type AttentionToastRequest =
  | { kind: "delivery"; delivery: AttentionDelivery; onActivate: () => void }
  /** The ledge folding a needs-you burst's older actions; `id` names the whole batch. */
  | { kind: "overflow"; id: string; count: number; onActivate: () => void }
  | {
      kind: "layout-upgrade";
      id: string;
      title: string;
      body: string;
      meta: string;
      onActivate: () => void;
    };

export type PresentAttentionToast = (request: AttentionToastRequest) => void;
