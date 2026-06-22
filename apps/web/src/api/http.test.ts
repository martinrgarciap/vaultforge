import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./ApiError";
import { requestJSON } from "./http";

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("requestJSON", () => {
  it("uses relative requests with cookies and JSON headers", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse({ value: "ok" }));

    vi.stubGlobal("fetch", fetchMock);

    const result = await requestJSON<{ value: string }>("/v1/example", {
      method: "POST",
      json: {
        example: true,
      },
    });

    expect(result).toEqual({ value: "ok" });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    const [path, request] = fetchMock.mock.calls[0];

    expect(path).toBe("/v1/example");
    expect(request?.credentials).toBe("include");
    expect(request?.body).toBe('{"example":true}');

    const headers = new Headers(request?.headers);

    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("does not add a content type to a bodyless request", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse({ refreshed: true }));

    vi.stubGlobal("fetch", fetchMock);

    await requestJSON("/v1/auth/refresh", {
      method: "POST",
    });

    const request = fetchMock.mock.calls[0][1];
    const headers = new Headers(request?.headers);

    expect(request?.body).toBeUndefined();
    expect(headers.has("Content-Type")).toBe(false);
  });

  it("supports 204 responses", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(null, {
        status: 204,
      }),
    );

    vi.stubGlobal("fetch", fetchMock);

    await expect(
      requestJSON<void>("/v1/sessions/current", {
        method: "DELETE",
      }),
    ).resolves.toBeUndefined();
  });

  it("creates an ApiError from a backend error envelope", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: "unauthorized",
            message: "A valid access token is required.",
            request_id: "request-123",
          },
        },
        401,
      ),
    );

    vi.stubGlobal("fetch", fetchMock);

    const promise = requestJSON("/v1/vaults");

    await expect(promise).rejects.toMatchObject({
      status: 401,
      code: "unauthorized",
      message: "A valid access token is required.",
      requestId: "request-123",
    });
  });

  it("uses a safe fallback for an invalid error response", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response("unexpected", {
        status: 500,
      }),
    );

    vi.stubGlobal("fetch", fetchMock);

    await expect(requestJSON("/v1/vaults")).rejects.toMatchObject({
      status: 500,
      code: "request_failed",
      message: "VaultForge is temporarily unavailable.",
    });
  });

  it("converts network failures into a safe ApiError", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new Error("private network detail"));

    vi.stubGlobal("fetch", fetchMock);

    const promise = requestJSON("/health/ready");

    await expect(promise).rejects.toBeInstanceOf(ApiError);
    await expect(promise).rejects.toMatchObject({
      status: 0,
      code: "network_error",
    });
  });

  it("rejects absolute or protocol-relative URLs", async () => {
    await expect(requestJSON("https://example.com/v1/vaults")).rejects.toThrow(
      "API requests must use a relative application path.",
    );

    await expect(requestJSON("//example.com/v1/vaults")).rejects.toThrow(
      "API requests must use a relative application path.",
    );
  });
});
