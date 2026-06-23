import { describe, expect, it } from "vitest";

import { ApiError } from "../api/ApiError";
import {
  itemTypes,
  parseItemListResponse,
  parseItemResponse,
} from "./contracts";
import {
  createItemIdempotencyKey,
  itemCollectionPath,
  itemCreatePath,
  itemPermanentDeletePath,
  itemResourcePath,
  itemRestorePath,
  itemVersionHeader,
} from "./request";
import { defaultPayloadForType, parseItemPayload } from "./validation";

import {
  itemFieldValue,
  itemPayloadObject,
  updateItemPayloadField,
} from "./form";

const itemResource = {
  id: "item-123",
  type: "api_key",
  payload: {
    name: "Synthetic API key",
    apiKey: "synthetic-value",
  },
  version: 1,
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

describe("item response contracts", () => {
  it("parses an active item response", () => {
    expect(
      parseItemResponse({
        item: itemResource,
      }),
    ).toEqual({
      item: itemResource,
    });
  });

  it("parses a deleted item and pagination cursor", () => {
    const deletedItem = {
      ...itemResource,
      deletedAt: "2026-06-22T13:00:00Z",
    };

    expect(
      parseItemListResponse({
        items: [deletedItem],
        nextCursor: "cursor-token",
      }),
    ).toEqual({
      items: [deletedItem],
      nextCursor: "cursor-token",
    });
  });

  it.each(itemTypes)("accepts the %s item type", (type) => {
    expect(
      parseItemResponse({
        item: {
          ...itemResource,
          type,
        },
      }).item.type,
    ).toBe(type);
  });

  it("rejects unknown item types", () => {
    expect(() =>
      parseItemResponse({
        item: {
          ...itemResource,
          type: "unknown",
        },
      }),
    ).toThrow(ApiError);
  });

  it("rejects array payloads", () => {
    expect(() =>
      parseItemResponse({
        item: {
          ...itemResource,
          payload: [],
        },
      }),
    ).toThrow(ApiError);
  });

  it("rejects invalid item versions", () => {
    expect(() =>
      parseItemResponse({
        item: {
          ...itemResource,
          version: 0,
        },
      }),
    ).toThrow(ApiError);
  });

  it("rejects empty pagination cursors", () => {
    expect(() =>
      parseItemListResponse({
        items: [],
        nextCursor: "",
      }),
    ).toThrow(ApiError);
  });
});

describe("item payload validation", () => {
  it("parses and formats a JSON object", () => {
    const result = parseItemPayload('{"name":"Synthetic","enabled":true}');

    expect(result).toEqual({
      ok: true,
      payload: {
        name: "Synthetic",
        enabled: true,
      },
      formatted: '{\n  "name": "Synthetic",\n  "enabled": true\n}',
    });
  });

  it.each([
    ["", "Item payload is required."],
    ["not-json", "Item payload must contain valid JSON."],
    ["[]", "Item payload must contain one JSON object."],
    ['"value"', "Item payload must contain one JSON object."],
    ["null", "Item payload must contain one JSON object."],
  ])("rejects invalid payload %j", (value, message) => {
    expect(parseItemPayload(value)).toEqual({
      ok: false,
      error: message,
    });
  });

  it("rejects payloads larger than 64 KiB", () => {
    const value = JSON.stringify({
      value: "a".repeat(64 * 1024),
    });

    expect(parseItemPayload(value)).toEqual({
      ok: false,
      error: "Item payload must not exceed 64 KiB.",
    });
  });

  it.each(itemTypes)("provides an object template for %s", (type) => {
    const payload = defaultPayloadForType(type);

    expect(payload).not.toBeNull();
    expect(Array.isArray(payload)).toBe(false);
    expect(typeof payload).toBe("object");
  });
});

describe("item request helpers", () => {
  it("builds active and paginated collection paths", () => {
    expect(itemCollectionPath("vault 123", "active")).toBe(
      "/v1/vaults/vault%20123/items?state=active&limit=20",
    );

    expect(itemCollectionPath("vault-123", "deleted", "cursor/value")).toBe(
      "/v1/vaults/vault-123/items?state=deleted&limit=20&after=cursor%2Fvalue",
    );

    expect(itemCreatePath("vault 123")).toBe("/v1/vaults/vault%20123/items");
  });

  it("builds item lifecycle paths", () => {
    expect(itemResourcePath("vault-123", "item-123", "deleted")).toBe(
      "/v1/vaults/vault-123/items/item-123?state=deleted",
    );

    expect(itemRestorePath("vault-123", "item-123")).toBe(
      "/v1/vaults/vault-123/items/item-123/restore",
    );

    expect(itemPermanentDeletePath("vault-123", "item-123")).toBe(
      "/v1/vaults/vault-123/items/item-123/permanent",
    );
  });

  it("formats a strong item-version header", () => {
    expect(itemVersionHeader(3)).toBe('"3"');
    expect(() => itemVersionHeader(0)).toThrow(TypeError);
  });

  it("creates distinct bounded idempotency keys", () => {
    const firstKey = createItemIdempotencyKey();
    const secondKey = createItemIdempotencyKey();

    expect(firstKey).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
    );
    expect(firstKey.length).toBeLessThanOrEqual(255);
    expect(secondKey).not.toBe(firstKey);
  });
});

describe("structured item forms", () => {
  it("updates a field while preserving unknown payload fields", () => {
    const result = updateItemPayloadField(
      "secure_note",
      JSON.stringify({
        title: "Original",
        note: "Synthetic note",
        customField: "preserved",
      }),
      "title",
      "Updated",
    );

    expect(JSON.parse(result)).toEqual({
      title: "Updated",
      note: "Synthetic note",
      customField: "preserved",
    });
  });

  it("converts database ports to numbers", () => {
    const result = updateItemPayloadField(
      "database_connection",
      JSON.stringify({
        name: "Synthetic database",
        port: 5432,
      }),
      "port",
      "6432",
    );

    expect(JSON.parse(result)).toEqual({
      name: "Synthetic database",
      port: 6432,
    });
  });

  it("falls back to the selected type template", () => {
    const payload = itemPayloadObject("secure_note", "invalid-json");

    expect(itemFieldValue(payload, "title")).toBe("Synthetic note");
  });
});
