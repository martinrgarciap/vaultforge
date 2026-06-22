const maximumVaultNameLength = 128;

const invalidVaultNameMessage =
  "Vault name must be valid Unicode, contain no control characters, and be between 1 and 128 characters.";

function hasUnpairedSurrogate(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const codeUnit = value.charCodeAt(index);

    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      const nextCodeUnit = value.charCodeAt(index + 1);

      if (nextCodeUnit < 0xdc00 || nextCodeUnit > 0xdfff) {
        return true;
      }

      index += 1;
      continue;
    }

    if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
      return true;
    }
  }

  return false;
}

export function normalizeVaultNameForSubmission(value: string): string {
  return value.normalize("NFC").trim();
}

export function validateVaultName(value: string): string | undefined {
  if (hasUnpairedSurrogate(value)) {
    return invalidVaultNameMessage;
  }

  const normalizedName = normalizeVaultNameForSubmission(value);

  if (
    normalizedName === "" ||
    /\p{Cc}/u.test(normalizedName) ||
    Array.from(normalizedName).length > maximumVaultNameLength
  ) {
    return invalidVaultNameMessage;
  }

  return undefined;
}
