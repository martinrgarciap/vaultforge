import type { ItemType, SyntheticItemPayload } from "./contracts";
import {
  defaultPayloadForType,
  formatItemPayload,
  parseItemPayload,
} from "./validation";

export type ItemFormFieldKind =
  | "text"
  | "password"
  | "url"
  | "number"
  | "multiline";

export interface ItemFormField {
  key: string;
  label: string;
  kind: ItemFormFieldKind;
  placeholder?: string;
  wide?: boolean;
}

const fieldsByType: Record<ItemType, readonly ItemFormField[]> = {
  login: [
    {
      key: "title",
      label: "title",
      kind: "text",
      placeholder: "Synthetic login",
    },
    {
      key: "username",
      label: "username",
      kind: "text",
      placeholder: "demo-user",
    },
    {
      key: "password",
      label: "password",
      kind: "password",
      placeholder: "synthetic-password",
    },
    {
      key: "website",
      label: "website",
      kind: "url",
      placeholder: "https://example.test",
    },
  ],

  api_key: [
    {
      key: "name",
      label: "name",
      kind: "text",
      placeholder: "Synthetic API key",
    },
    {
      key: "service",
      label: "service",
      kind: "text",
      placeholder: "Development service",
    },
    {
      key: "apiKey",
      label: "API key",
      kind: "password",
      placeholder: "synthetic-api-key",
    },
  ],

  environment_variable: [
    {
      key: "name",
      label: "variable name",
      kind: "text",
      placeholder: "SYNTHETIC_API_KEY",
    },
    {
      key: "value",
      label: "value",
      kind: "password",
      placeholder: "synthetic-value",
    },
  ],

  database_connection: [
    {
      key: "name",
      label: "name",
      kind: "text",
      placeholder: "Synthetic database",
    },
    {
      key: "host",
      label: "host",
      kind: "text",
      placeholder: "localhost",
    },
    {
      key: "port",
      label: "port",
      kind: "number",
      placeholder: "5432",
    },
    {
      key: "database",
      label: "database",
      kind: "text",
      placeholder: "vaultforge_dev",
    },
    {
      key: "username",
      label: "username",
      kind: "text",
      placeholder: "demo-user",
    },
    {
      key: "password",
      label: "password",
      kind: "password",
      placeholder: "synthetic-password",
    },
  ],

  secure_note: [
    {
      key: "title",
      label: "title",
      kind: "text",
      placeholder: "Synthetic note",
      wide: true,
    },
    {
      key: "note",
      label: "note",
      kind: "multiline",
      placeholder: "Synthetic content only.",
      wide: true,
    },
  ],
};

export function itemFormFields(type: ItemType): readonly ItemFormField[] {
  return fieldsByType[type];
}

export function itemPayloadObject(
  type: ItemType,
  value: string,
): SyntheticItemPayload {
  const parsedPayload = parseItemPayload(value);

  if (parsedPayload.ok) {
    return parsedPayload.payload;
  }

  return defaultPayloadForType(type);
}

export function itemFieldValue(
  payload: SyntheticItemPayload,
  key: string,
): string {
  const value = payload[key];

  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }

  return "";
}

export function updateItemPayloadField(
  type: ItemType,
  currentValue: string,
  fieldKey: string,
  fieldValue: string,
): string {
  const field = itemFormFields(type).find(
    (candidate) => candidate.key === fieldKey,
  );

  if (!field) {
    throw new TypeError(`Unknown structured field: ${fieldKey}`);
  }

  let normalizedValue: unknown = fieldValue;

  if (field.kind === "number") {
    const trimmedValue = fieldValue.trim();

    if (trimmedValue === "") {
      normalizedValue = "";
    } else {
      const numericValue = Number(trimmedValue);

      normalizedValue = Number.isFinite(numericValue)
        ? numericValue
        : fieldValue;
    }
  }

  const currentPayload = itemPayloadObject(type, currentValue);

  return formatItemPayload({
    ...currentPayload,
    [fieldKey]: normalizedValue,
  });
}
