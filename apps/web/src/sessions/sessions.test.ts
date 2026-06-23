import { describe, expect, it } from "vitest";

import { ApiError } from "../api/ApiError";
import { parseSessionListResponse } from "./contracts";
import { sessionClientLabel, sessionUserAgentDescription } from "./display";
import {
  currentSessionPath,
  sessionResourcePath,
  sessionsPath,
} from "./request";

const validSession = {
  id: "session-123",
  userAgent: "Thunder Client",
  createdAt: "2026-06-22T12:00:00Z",
  expiresAt: "2026-07-22T12:00:00Z",
  current: true,
};

describe("session contracts", () => {
  it("parses a valid session list", () => {
    expect(
      parseSessionListResponse({
        sessions: [validSession],
      }),
    ).toEqual({
      sessions: [validSession],
    });
  });

  it.each([
    undefined,
    null,
    {},
    {
      sessions: "invalid",
    },
    {
      sessions: [
        {
          ...validSession,
          id: "",
        },
      ],
    },
    {
      sessions: [
        {
          ...validSession,
          current: "true",
        },
      ],
    },
    {
      sessions: [
        {
          ...validSession,
          createdAt: "not-a-date",
        },
      ],
    },
    {
      sessions: [
        {
          ...validSession,
          expiresAt: validSession.createdAt,
        },
      ],
    },
  ])("rejects malformed session responses", (value) => {
    expect(() => {
      parseSessionListResponse(value);
    }).toThrow(ApiError);
  });

  it("rejects duplicate session IDs", () => {
    expect(() => {
      parseSessionListResponse({
        sessions: [
          validSession,
          {
            ...validSession,
            current: false,
          },
        ],
      });
    }).toThrow(ApiError);
  });

  it("rejects more than one current session", () => {
    expect(() => {
      parseSessionListResponse({
        sessions: [
          validSession,
          {
            ...validSession,
            id: "session-456",
          },
        ],
      });
    }).toThrow(ApiError);
  });
});

describe("session display helpers", () => {
  it.each([
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/148.0.0.0 Safari/537.36",
      "Chrome on macOS",
    ],
    [
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0",
      "Firefox on Windows",
    ],
    ["Thunder Client", "Thunder Client"],
    ["", "Unknown client"],
  ])("formats a readable client label", (userAgent, expected) => {
    expect(sessionClientLabel(userAgent)).toBe(expected);
  });

  it("provides a fallback user-agent description", () => {
    expect(sessionUserAgentDescription("")).toBe(
      "User agent was not provided.",
    );
  });
});

describe("session request helpers", () => {
  it("provides collection and current-session paths", () => {
    expect(sessionsPath).toBe("/v1/sessions");
    expect(currentSessionPath).toBe("/v1/sessions/current");
  });

  it("encodes targeted session IDs", () => {
    expect(sessionResourcePath("session/value")).toBe(
      "/v1/sessions/session%2Fvalue",
    );
  });

  it.each(["", " session-123", "session-123 "])(
    "rejects invalid session IDs",
    (sessionId) => {
      expect(() => {
        sessionResourcePath(sessionId);
      }).toThrow(TypeError);
    },
  );
});
