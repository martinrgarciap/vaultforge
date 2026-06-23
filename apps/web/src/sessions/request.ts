export const sessionsPath = "/v1/sessions";
export const currentSessionPath = "/v1/sessions/current";

export function sessionResourcePath(sessionId: string): string {
  if (sessionId === "" || sessionId.trim() !== sessionId) {
    throw new TypeError("Session ID must be a non-empty identifier.");
  }

  return `${sessionsPath}/${encodeURIComponent(sessionId)}`;
}
