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
  required?: boolean;
  wide?: boolean;
}

const fieldsByType: Record<ItemType, readonly ItemFormField[]> = {
  login: [
    {
      key: "title",
      label: "Title",
      kind: "text",
      placeholder: "Synthetic login",
      required: true,
    },
    {
      key: "username",
      label: "Username",
      kind: "text",
      placeholder: "demo-user",
      required: true,
    },
    {
      key: "password",
      label: "Password",
      kind: "password",
      placeholder: "synthetic-password",
      required: true,
    },
    {
      key: "website",
      label: "Website",
      kind: "url",
      placeholder: "https://example.test",
    },
  ],

  api_key: [
    {
      key: "name",
      label: "Name",
      kind: "text",
      placeholder: "Synthetic API key",
      required: true,
    },
    {
      key: "service",
      label: "Service",
      kind: "text",
      placeholder: "Development service",
      required: true,
    },
    {
      key: "apiKey",
      label: "API key",
      kind: "password",
      placeholder: "synthetic-api-key",
      required: true,
    },
  ],

  environment_variable: [
    {
      key: "name",
      label: "Variable name",
      kind: "text",
      placeholder: "SYNTHETIC_API_KEY",
      required: true,
    },
    {
      key: "value",
      label: "Value",
      kind: "password",
      placeholder: "synthetic-value",
      required: true,
    },
  ],

  database_connection: [
    {
      key: "name",
      label: "Name",
      kind: "text",
      placeholder: "Synthetic database",
      required: true,
    },
    {
      key: "host",
      label: "Host",
      kind: "text",
      placeholder: "localhost",
      required: true,
    },
    {
      key: "port",
      label: "Port",
      kind: "number",
      placeholder: "5432",
      required: true,
    },
    {
      key: "database",
      label: "Database",
      kind: "text",
      placeholder: "vaultforge_dev",
      required: true,
    },
    {
      key: "username",
      label: "Username",
      kind: "text",
      placeholder: "demo-user",
      required: true,
    },
    {
      key: "password",
      label: "Password",
      kind: "password",
      placeholder: "synthetic-password",
      required: true,
    },
  ],

  secure_note: [
    {
      key: "title",
      label: "Title",
      kind: "text",
      placeholder: "Synthetic note",
      required: true,
      wide: true,
    },
    {
      key: "note",
      label: "Note",
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

export function requiredItemFieldErrors(
  type: ItemType,
  payload: SyntheticItemPayload,
): Record<string, string> {
  return Object.fromEntries(
    itemFormFields(type)
      .filter((field) => {
        if (!field.required) {
          return false;
        }

        const value = payload[field.key];

        if (typeof value === "number") {
          return !Number.isFinite(value);
        }

        return typeof value !== "string" || value.trim() === "";
      })
      .map((field) => [field.key, `${field.label} is required.`]),
  );
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
