import { ApiError } from "./ApiError";
import type { ApiRequestOptions } from "./types";

function requireRelativePath(path: string): void {
  if (!path.startsWith("/") || path.startsWith("//")) {
    throw new TypeError("API requests must use a relative application path.");
  }
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

async function readSuccessResponse<T>(response: Response): Promise<T> {
  if (response.status === 204 || response.status === 205) {
    return undefined as T;
  }

  const text = await response.text();

  if (text.trim() === "") {
    return undefined as T;
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    throw ApiError.invalidResponse();
  }
}

export async function requestJSON<T = void>(
  path: string,
  options: ApiRequestOptions = {},
): Promise<T> {
  requireRelativePath(path);

  const { headers: suppliedHeaders, json, ...requestOptions } = options;

  const headers = new Headers(suppliedHeaders);

  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  let body: string | undefined;

  if (json !== undefined) {
    const serializedBody = JSON.stringify(json);

    if (serializedBody === undefined) {
      throw new TypeError("The JSON request body could not be serialized.");
    }

    body = serializedBody;

    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  let response: Response;

  try {
    response = await fetch(path, {
      ...requestOptions,
      credentials: "include",
      headers,
      body,
    });
  } catch (error) {
    if (isAbortError(error)) {
      throw error;
    }

    throw ApiError.network();
  }

  if (!response.ok) {
    throw await ApiError.fromResponse(response);
  }

  return readSuccessResponse<T>(response);
}
