import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { ProfileMutationScopeParams, ProfileScopeParams } from "@/systems/profiles";

import { BridgesApiError } from "./bridges-api";
import type {
  BridgeVerifyResponse,
  BridgeWebhookRegistrationResponse,
  SendBridgeTestRequest,
  SendBridgeTestResponse,
  SlackBridgeManifestResponse,
} from "../types";

export async function getSlackBridgeManifest(
  instanceID: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<SlackBridgeManifestResponse> {
  const { data, error, response } = await apiClient.GET("/api/bridges/providers/slack/manifest", {
    params: { query: { instance: instanceID, ...scope } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(
        `Failed to load Slack manifest for bridge "${instanceID}"`,
        response,
        error
      ),
      response.status
    );
  }

  return requireResponseData(
    data,
    response,
    `Failed to load Slack manifest for bridge "${instanceID}"`
  );
}

export async function verifyBridge(
  id: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<BridgeVerifyResponse> {
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/verify", {
    params: { path: { id }, query: { ...scope } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to verify bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to verify bridge "${id}"`);
}

export async function sendBridgeTest(
  id: string,
  body: SendBridgeTestRequest,
  scope: ProfileMutationScopeParams,
  signal?: AbortSignal
): Promise<SendBridgeTestResponse> {
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/send-test", {
    body,
    params: { path: { id }, query: { ...scope } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to send a test through bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to send a test through bridge "${id}"`);
}

export async function registerBridgeWebhook(
  id: string,
  scope: ProfileMutationScopeParams,
  signal?: AbortSignal
): Promise<BridgeWebhookRegistrationResponse> {
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/webhook/register", {
    params: { path: { id }, query: { ...scope } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to register the webhook for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to register the webhook for bridge "${id}"`);
}
