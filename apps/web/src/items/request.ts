import type { ItemState } from "./contracts";

export const itemPageLimit = 20;

function requireIdentifier(value: string, label: string): string {
  if (value === "" || value.trim() !== value) {
    throw new TypeError(`${label} must be a non-empty identifier.`);
  }

  return encodeURIComponent(value);
}

export function itemCreatePath(vaultId: string): string {
  const encodedVaultId = requireIdentifier(vaultId, "Vault ID");

  return `/v1/vaults/${encodedVaultId}/items`;
}

export function itemCollectionPath(
  vaultId: string,
  state: ItemState,
  after?: string,
): string {
  const query = new URLSearchParams({
    state,
    limit: String(itemPageLimit),
  });

  if (after) {
    query.set("after", after);
  }

  return `${itemCreatePath(vaultId)}?${query.toString()}`;
}

export function itemResourcePath(
  vaultId: string,
  itemId: string,
  state?: ItemState,
): string {
  const encodedVaultId = requireIdentifier(vaultId, "Vault ID");
  const encodedItemId = requireIdentifier(itemId, "Item ID");

  const path = `/v1/vaults/${encodedVaultId}/items/${encodedItemId}`;

  if (!state) {
    return path;
  }

  return `${path}?state=${encodeURIComponent(state)}`;
}

export function itemRestorePath(vaultId: string, itemId: string): string {
  return `${itemResourcePath(vaultId, itemId)}/restore`;
}

export function itemPermanentDeletePath(
  vaultId: string,
  itemId: string,
): string {
  return `${itemResourcePath(vaultId, itemId)}/permanent`;
}

export function itemVersionHeader(version: number): string {
  if (!Number.isInteger(version) || version < 1) {
    throw new TypeError("Item version must be a positive integer.");
  }

  return `"${version}"`;
}

function fallbackUUID(): string {
  const bytes = new Uint8Array(16);

  globalThis.crypto.getRandomValues(bytes);

  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  const hex = Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");

  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join("-");
}

export function createItemIdempotencyKey(): string {
  if (typeof globalThis.crypto.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }

  return fallbackUUID();
}
