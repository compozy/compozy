/** The real attention toast, carrying the agent's face instead of a session glyph (S12). */
import { ToastFrame } from "@/systems/os/components/attention-toast";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { INSTANCES, SCOPE_LABEL } from "../stays-on-agents-shared/fixtures";
import { openMainSession } from "./open-main";

const instance = INSTANCES.triageCompozy;

export function NeedsYouToast() {
  return (
    <div className="absolute top-14 right-4 z-30 w-80">
      <ToastFrame
        mark={
          <AgentAvatar
            appearance={instance.agent.appearance}
            label={instance.agent.label}
            size={32}
            state="needs-you"
            surfaceClassName="ring-popover"
          />
        }
        body="Has 2 suggestions for you"
        meta={SCOPE_LABEL(instance.scope)}
        onActivate={() => openMainSession(instance)}
        testId="os-attention-toast-needs-you"
        title={`${instance.agent.label} needs you`}
      />
    </div>
  );
}
