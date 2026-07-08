import init, {
  decrypt,
  derive_key,
  encrypt,
  generate_key,
  unwrap_key,
  wrap_key,
} from "../../../../packages/crypto-wasm/pkg/crypto_wasm.js";

import type { WasmCryptoModule } from "./wasmCryptoProvider";

let modulePromise: Promise<WasmCryptoModule> | null = null;

export function loadCryptoWasm(): Promise<WasmCryptoModule> {
  modulePromise ??= initializeCryptoWasm();
  return modulePromise;
}

async function initializeCryptoWasm(): Promise<WasmCryptoModule> {
  await init();

  return {
    decrypt,
    derive_key,
    encrypt,
    generate_key,
    unwrap_key,
    wrap_key,
  };
}
