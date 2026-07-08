import { describe, expect, it } from "vitest";

import { base64ToBytes, bytesToBase64 } from "./encoding";

describe("base64 byte encoding", () => {
  it("round trips arbitrary bytes", () => {
    const bytes = Uint8Array.from([0, 1, 2, 3, 127, 128, 254, 255]);

    const encoded = bytesToBase64(bytes);
    const decoded = base64ToBytes(encoded);

    expect(Array.from(decoded)).toEqual(Array.from(bytes));
  });

  it("handles empty byte arrays", () => {
    const encoded = bytesToBase64(new Uint8Array());

    expect(encoded).toBe("");
    expect(Array.from(base64ToBytes(encoded))).toEqual([]);
  });

  it("handles larger byte arrays", () => {
    const bytes = Uint8Array.from(
      { length: 70_000 },
      (_, index) => index % 256,
    );

    const decoded = base64ToBytes(bytesToBase64(bytes));

    expect(Array.from(decoded)).toEqual(Array.from(bytes));
  });

  it.each(["not base64", "abc", "abcd=", "%%%%", " YWJj", "YWJj "])(
    "rejects malformed base64 value %s",
    (value) => {
      expect(() => base64ToBytes(value)).toThrow(TypeError);
    },
  );
});
