import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { BridgesApiError } from "./bridges-api";
import type {
  BridgeVerifyResponse,
  BridgeWebhookRegistrationResponse,
  SendBridgeTestRequest,
  SendBridgeTestResponse,
  SlackBridgeManifestResponse,
} from "../types";

function normalizedBridgeID(id: string): string {
  return id.trim();
}

export async function getSlackBridgeManifest(
  instanceID: string,
  signal?: AbortSignal
): Promise<SlackBridgeManifestResponse> {
  const instance = normalizedBridgeID(instanceID);
  const { data, error, response } = await apiClient.GET("/api/bridges/providers/slack/manifest", {
    params: { query: { instance } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(
        `Failed to load Slack manifest for bridge "${instance}"`,
        response,
        error
      ),
      response.status
    );
  }

  return requireResponseData(
    data,
    response,
    `Failed to load Slack manifest for bridge "${instance}"`
  );
}

export async function verifyBridge(
  id: string,
  signal?: AbortSignal
): Promise<BridgeVerifyResponse> {
  const bridgeID = normalizedBridgeID(id);
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/verify", {
    params: { path: { id: bridgeID } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to verify bridge "${bridgeID}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to verify bridge "${bridgeID}"`);
}

export async function sendBridgeTest(
  id: string,
  body: SendBridgeTestRequest,
  signal?: AbortSignal
): Promise<SendBridgeTestResponse> {
  const bridgeID = normalizedBridgeID(id);
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/send-test", {
    body,
    params: { path: { id: bridgeID } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to send a test through bridge "${bridgeID}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to send a test through bridge "${bridgeID}"`);
}

export async function registerBridgeWebhook(
  id: string,
  signal?: AbortSignal
): Promise<BridgeWebhookRegistrationResponse> {
  const bridgeID = normalizedBridgeID(id);
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/webhook/register", {
    params: { path: { id: bridgeID } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(
        `Failed to register the webhook for bridge "${bridgeID}"`,
        response,
        error
      ),
      response.status
    );
  }

  return requireResponseData(
    data,
    response,
    `Failed to register the webhook for bridge "${bridgeID}"`
  );
}
