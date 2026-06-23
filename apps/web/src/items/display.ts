import type { SyntheticItemPayload, VaultItem } from "./contracts";
import { itemTypeLabel } from "./validation";

function stringField(
  payload: SyntheticItemPayload,
  key: string,
): string | undefined {
  const value = payload[key];

  if (typeof value !== "string") {
    return undefined;
  }

  const normalizedValue = value.trim();

  return normalizedValue === "" ? undefined : normalizedValue;
}

function numberField(
  payload: SyntheticItemPayload,
  key: string,
): number | undefined {
  const value = payload[key];

  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

export function itemDisplayName(item: VaultItem): string {
  const preferredFields = ["title", "name", "label", "username"];

  for (const field of preferredFields) {
    const value = stringField(item.payload, field);

    if (value) {
      return value;
    }
  }

  return `${itemTypeLabel(item.type)} item`;
}

export function itemSummary(item: VaultItem): string {
  switch (item.type) {
    case "login":
      return (
        stringField(item.payload, "username") ??
        stringField(item.payload, "website") ??
        "Login credentials"
      );

    case "api_key":
      return stringField(item.payload, "service") ?? "API credentials";

    case "environment_variable":
      return stringField(item.payload, "name") ?? "Environment variable";

    case "database_connection": {
      const database = stringField(item.payload, "database");
      const host = stringField(item.payload, "host");
      const port = numberField(item.payload, "port");

      if (database && host) {
        return `${database} on ${host}${port ? `:${port}` : ""}`;
      }

      return host ?? database ?? "Database connection";
    }

    case "secure_note": {
      const note = stringField(item.payload, "note");

      if (!note) {
        return "Secure note";
      }

      return note.length > 72 ? `${note.slice(0, 69)}...` : note;
    }
  }
}

export function formatTimestamp(value: string): string {
  const timestamp = new Date(value);

  if (Number.isNaN(timestamp.getTime())) {
    return value;
  }

  return timestamp.toLocaleString();
}
