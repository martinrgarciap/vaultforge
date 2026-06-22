import type { ApiErrorEnvelope } from "./types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isApiErrorEnvelope(value: unknown): value is ApiErrorEnvelope {
  if (!isRecord(value) || !isRecord(value.error)) {
    return false;
  }

  return (
    typeof value.error.code === "string" &&
    typeof value.error.message === "string" &&
    (value.error.request_id === undefined ||
      typeof value.error.request_id === "string")
  );
}

function fallbackMessage(status: number): string {
  if (status === 401) {
    return "Authentication is required.";
  }

  if (status === 403) {
    return "The request is not permitted.";
  }

  if (status >= 500) {
    return "VaultForge is temporarily unavailable.";
  }

  return "The request could not be completed.";
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;

  constructor(
    status: number,
    code: string,
    message: string,
    requestId?: string,
  ) {
    super(message);

    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }

  static async fromResponse(response: Response): Promise<ApiError> {
    let value: unknown;

    try {
      const text = await response.text();

      if (text.trim() !== "") {
        value = JSON.parse(text) as unknown;
      }
    } catch {
      value = undefined;
    }

    if (isApiErrorEnvelope(value)) {
      return new ApiError(
        response.status,
        value.error.code,
        value.error.message,
        value.error.request_id,
      );
    }

    return new ApiError(
      response.status,
      "request_failed",
      fallbackMessage(response.status),
    );
  }

  static network(): ApiError {
    return new ApiError(
      0,
      "network_error",
      "Unable to reach VaultForge. Please try again.",
    );
  }

  static invalidResponse(): ApiError {
    return new ApiError(
      502,
      "invalid_response",
      "VaultForge returned an unexpected response.",
    );
  }
}
