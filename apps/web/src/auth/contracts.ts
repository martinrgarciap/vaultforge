import { ApiError } from "../api/ApiError";
import type { Account, LoginResponse, RefreshResponse } from "../api/types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function parseAccount(value: unknown): Account {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    typeof value.email !== "string" ||
    typeof value.status !== "string" ||
    typeof value.createdAt !== "string" ||
    typeof value.updatedAt !== "string"
  ) {
    throw ApiError.invalidResponse();
  }

  return {
    id: value.id,
    email: value.email,
    status: value.status,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  };
}

function requireString(value: Record<string, unknown>, field: string): string {
  const fieldValue = value[field];

  if (typeof fieldValue !== "string" || fieldValue.length === 0) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

export function parseLoginResponse(value: unknown): LoginResponse {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    user: parseAccount(value.user),
    tokenType: requireString(value, "tokenType"),
    accessToken: requireString(value, "accessToken"),
    accessTokenExpiresAt: requireString(value, "accessTokenExpiresAt"),
    refreshTokenExpiresAt: requireString(value, "refreshTokenExpiresAt"),
  };
}

export function parseRefreshResponse(value: unknown): RefreshResponse {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    tokenType: requireString(value, "tokenType"),
    accessToken: requireString(value, "accessToken"),
    accessTokenExpiresAt: requireString(value, "accessTokenExpiresAt"),
    refreshTokenExpiresAt: requireString(value, "refreshTokenExpiresAt"),
  };
}
