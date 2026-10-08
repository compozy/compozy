import { useAutomationDetailPage } from "../automation/use-automation-page";
import {
  AutomationDetailPanel,
  AutomationEditorDialog,
  type AutomationDetailRouteSearch,
} from "@/systems/automation";

interface AutomationDetailLocationProps {
  id: string;
  kind: "job" | "trigger";
  search: AutomationDetailRouteSearch;
}

/** Detail of one automation; routes keep the daemon entity (`jobs` | `triggers`). */
export function AutomationDetailLocation({ id, kind, search }: AutomationDetailLocationProps) {
  const page = useAutomationDetailPage(kind, id, search);
  return (
    <>
      <AutomationDetailPanel {...page.panel} />
      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
