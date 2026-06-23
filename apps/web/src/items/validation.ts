import type { ItemType, SyntheticItemPayload } from "./contracts";

const maximumPayloadBytes = 64 * 1024;

export const itemTypeOptions: ReadonlyArray<{
  value: ItemType;
  label: string;
}> = [
  {
    value: "login",
    label: "Login",
  },
  {
    value: "api_key",
    label: "API key",
  },
  {
    value: "environment_variable",
    label: "Environment variable",
  },
  {
    value: "database_connection",
    label: "Database connection",
  },
  {
    value: "secure_note",
    label: "Secure note",
  },
];

export type ParsedItemPayload =
  | {
      ok: true;
      payload: SyntheticItemPayload;
      formatted: string;
    }
  | {
      ok: false;
      error: string;
    };

export function itemTypeLabel(type: ItemType): string {
  return itemTypeOptions.find((option) => option.value === type)?.label ?? type;
}

export function formatItemPayload(payload: SyntheticItemPayload): string {
  return JSON.stringify(payload, null, 2);
}

export function defaultPayloadForType(type: ItemType): SyntheticItemPayload {
  switch (type) {
    case "login":
      return {
        title: "Synthetic login",
        username: "demo-user",
        password: "synthetic-password",
      };

    case "api_key":
      return {
        name: "Synthetic API key",
        apiKey: "synthetic-api-key",
      };

    case "environment_variable":
      return {
        name: "SYNTHETIC_API_KEY",
        value: "synthetic-value",
      };

    case "database_connection":
      return {
        name: "Synthetic database",
        host: "localhost",
        port: 5432,
        username: "demo-user",
        password: "synthetic-password",
      };

    case "secure_note":
      return {
        title: "Synthetic note",
        note: "Synthetic content only.",
      };
  }
}

export function parseItemPayload(value: string): ParsedItemPayload {
  const trimmedValue = value.trim();

  if (trimmedValue === "") {
    return {
      ok: false,
      error: "Item payload is required.",
    };
  }

  if (new TextEncoder().encode(trimmedValue).byteLength > maximumPayloadBytes) {
    return {
      ok: false,
      error: "Item payload must not exceed 64 KiB.",
    };
  }

  let parsedValue: unknown;

  try {
    parsedValue = JSON.parse(trimmedValue) as unknown;
  } catch {
    return {
      ok: false,
      error: "Item payload must contain valid JSON.",
    };
  }

  if (
    typeof parsedValue !== "object" ||
    parsedValue === null ||
    Array.isArray(parsedValue)
  ) {
    return {
      ok: false,
      error: "Item payload must contain one JSON object.",
    };
  }

  const payload = parsedValue as SyntheticItemPayload;

  return {
    ok: true,
    payload,
    formatted: formatItemPayload(payload),
  };
}
