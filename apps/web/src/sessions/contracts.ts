import { ApiError } from "../api/ApiError";

export interface SessionSummary {
  id: string;
  userAgent: string;
  createdAt: string;
  expiresAt: string;
  current: boolean;
}

export interface SessionListResponse {
  sessions: SessionSummary[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function requireString(
  value: Record<string, unknown>,
  field: string,
  allowEmpty = false,
): string {
  const fieldValue = value[field];

  if (typeof fieldValue !== "string") {
    throw ApiError.invalidResponse();
  }

  if (
    !allowEmpty &&
    (fieldValue.length === 0 || fieldValue.trim() !== fieldValue)
  ) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function requireTimestamp(
  value: Record<string, unknown>,
  field: string,
): string {
  const timestamp = requireString(value, field);

  if (Number.isNaN(Date.parse(timestamp))) {
    throw ApiError.invalidResponse();
  }

  return timestamp;
}

function requireBoolean(
  value: Record<string, unknown>,
  field: string,
): boolean {
  const fieldValue = value[field];

  if (typeof fieldValue !== "boolean") {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function parseSessionSummary(value: unknown): SessionSummary {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  const createdAt = requireTimestamp(value, "createdAt");
  const expiresAt = requireTimestamp(value, "expiresAt");

  if (Date.parse(expiresAt) <= Date.parse(createdAt)) {
    throw ApiError.invalidResponse();
  }

  return {
    id: requireString(value, "id"),
    userAgent: requireString(value, "userAgent", true),
    createdAt,
    expiresAt,
    current: requireBoolean(value, "current"),
  };
}

export function parseSessionListResponse(value: unknown): SessionListResponse {
  if (!isRecord(value) || !Array.isArray(value.sessions)) {
    throw ApiError.invalidResponse();
  }

  const sessions = value.sessions.map(parseSessionSummary);
  const sessionIDs = new Set<string>();
  let currentSessionCount = 0;

  for (const session of sessions) {
    if (sessionIDs.has(session.id)) {
      throw ApiError.invalidResponse();
    }

    sessionIDs.add(session.id);

    if (session.current) {
      currentSessionCount += 1;
    }
  }

  if (currentSessionCount > 1) {
    throw ApiError.invalidResponse();
  }

  return {
    sessions,
  };
}
