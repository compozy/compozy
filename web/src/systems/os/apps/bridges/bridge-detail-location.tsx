import { useBridgeDetailPage } from "./use-bridge-detail-page";
import { BridgeDetailPanel, BridgeEditDialog } from "@/systems/bridges";

export function BridgeDetailLocation({ id }: { id: string }) {
  const page = useBridgeDetailPage(id);

  return (
    <>
      <BridgeDetailPanel {...page.detailPanelProps} />
      <BridgeEditDialog {...page.editDialogProps} />
    </>
  );
}
