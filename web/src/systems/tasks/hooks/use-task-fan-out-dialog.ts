import { useState, type FormEvent } from "react";

import {
  DEFAULT_NETWORK_PARTICIPATION_DRAFT,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/systems/network";

import type { FanOutTaskRunsRequest } from "../types";

const FAN_OUT_NETWORK_STRATEGIES = ["named", "run"] as const;

export interface UseTaskFanOutDialogParams {
  onOpenChange: (open: boolean) => void;
  onFanOut: (data: FanOutTaskRunsRequest) => Promise<void>;
}

function parseDesignations(value: string): FanOutTaskRunsRequest["designations"] {
  return value
    .split(/\r?\n/u)
    .map(line => line.trim())
    .filter(line => line.length > 0)
    .map(brief => ({ brief }));
}

/** Controlled form state for the fan-out dialog opened from the task head overflow. */
export function useTaskFanOutDialog({ onOpenChange, onFanOut }: UseTaskFanOutDialogParams) {
  const [designationsText, setDesignationsText] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [networkParticipation, setNetworkParticipation] = useState<NetworkParticipationDraft>({
    ...DEFAULT_NETWORK_PARTICIPATION_DRAFT,
  });

  const handleOpenChange = (nextOpen: boolean) => {
    onOpenChange(nextOpen);
    if (!nextOpen) {
      setFormError(null);
      setDesignationsText("");
      setNetworkParticipation({ ...DEFAULT_NETWORK_PARTICIPATION_DRAFT });
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    const designations = parseDesignations(designationsText);
    if (designations.length === 0) {
      setFormError("Add at least one assignment.");
      return;
    }
    const participationError = networkParticipationValidationMessage(
      networkParticipation,
      FAN_OUT_NETWORK_STRATEGIES
    );
    if (participationError) {
      setFormError(participationError);
      return;
    }

    const payload: FanOutTaskRunsRequest = {
      designations,
      network_participation: serializeNetworkParticipation(networkParticipation),
    };

    try {
      await onFanOut(payload);
      handleOpenChange(false);
    } catch {
      // The route hook owns the toast; keep the dialog open for correction.
    }
  };

  return {
    designationsText,
    formError,
    handleOpenChange,
    handleSubmit,
    networkParticipation,
    networkStrategies: FAN_OUT_NETWORK_STRATEGIES,
    setDesignationsText,
    setNetworkParticipation,
  };
}
