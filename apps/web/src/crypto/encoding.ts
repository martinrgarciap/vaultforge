const base64Pattern =
  /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

const chunkSize = 0x8000;

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";

  for (let index = 0; index < bytes.length; index += chunkSize) {
    const chunk = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...chunk);
  }

  return btoa(binary);
}

export function base64ToBytes(value: string, label = "value"): Uint8Array {
  if (!isValidBase64(value)) {
    throw new TypeError(`${label} must be valid base64.`);
  }

  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);

  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }

  return bytes;
}

function isValidBase64(value: string): boolean {
  if (value.length === 0) {
    return true;
  }

  if (value.trim() !== value) {
    return false;
  }

  if (value.length % 4 !== 0) {
    return false;
  }

  return base64Pattern.test(value);
}
