/* tslint:disable */
/* eslint-disable */

/**
 * Decrypt a [nonce][ciphertext+tag] blob with a 32-byte key.
 */
export function decrypt(key: Uint8Array, blob: Uint8Array): Uint8Array;

/**
 * Derive a 32-byte vault key from a passphrase + salt (Argon2id).
 */
export function derive_key(passphrase: Uint8Array, salt: Uint8Array): Uint8Array;

/**
 * Encrypt plaintext with a 32-byte key → [nonce][ciphertext+tag].
 */
export function encrypt(key: Uint8Array, plaintext: Uint8Array): Uint8Array;

/**
 * Generate a fresh random 32-byte key.
 */
export function generate_key(): Uint8Array;

/**
 * Unwrap (decrypt) a wrapped vault key with a KEK.
 */
export function unwrap_key(kek: Uint8Array, wrapped: Uint8Array): Uint8Array;

/**
 * Wrap (encrypt) a 32-byte vault key with a KEK.
 */
export function wrap_key(kek: Uint8Array, vault_key: Uint8Array): Uint8Array;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
    readonly memory: WebAssembly.Memory;
    readonly decrypt: (a: number, b: number, c: number, d: number) => [number, number, number, number];
    readonly derive_key: (a: number, b: number, c: number, d: number) => [number, number, number, number];
    readonly encrypt: (a: number, b: number, c: number, d: number) => [number, number, number, number];
    readonly generate_key: () => [number, number, number, number];
    readonly unwrap_key: (a: number, b: number, c: number, d: number) => [number, number, number, number];
    readonly wrap_key: (a: number, b: number, c: number, d: number) => [number, number, number, number];
    readonly __wbindgen_exn_store: (a: number) => void;
    readonly __externref_table_alloc: () => number;
    readonly __wbindgen_externrefs: WebAssembly.Table;
    readonly __wbindgen_malloc: (a: number, b: number) => number;
    readonly __externref_table_dealloc: (a: number) => void;
    readonly __wbindgen_free: (a: number, b: number, c: number) => void;
    readonly __wbindgen_start: () => void;
}

export type SyncInitInput = BufferSource | WebAssembly.Module;

/**
 * Instantiates the given `module`, which can either be bytes or
 * a precompiled `WebAssembly.Module`.
 *
 * @param {{ module: SyncInitInput }} module - Passing `SyncInitInput` directly is deprecated.
 *
 * @returns {InitOutput}
 */
export function initSync(module: { module: SyncInitInput } | SyncInitInput): InitOutput;

/**
 * If `module_or_path` is {RequestInfo} or {URL}, makes a request and
 * for everything else, calls `WebAssembly.instantiate` directly.
 *
 * @param {{ module_or_path: InitInput | Promise<InitInput> }} module_or_path - Passing `InitInput` directly is deprecated.
 *
 * @returns {Promise<InitOutput>}
 */
export default function __wbg_init (module_or_path?: { module_or_path: InitInput | Promise<InitInput> } | InitInput | Promise<InitInput>): Promise<InitOutput>;
