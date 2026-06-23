export interface LoginNavigationState {
  registrationComplete: boolean;
  email?: string;
  authenticationRequired: boolean;
  returnTo: string;
}

const defaultReturnTo = "/vaults";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isSafeReturnTo(value: unknown): value is string {
  if (
    typeof value !== "string" ||
    !value.startsWith("/") ||
    value.startsWith("//")
  ) {
    return false;
  }

  const path = value.split(/[?#]/, 1)[0];

  return path !== "/login" && path !== "/register";
}

export function readLoginNavigationState(value: unknown): LoginNavigationState {
  if (!isRecord(value)) {
    return {
      registrationComplete: false,
      authenticationRequired: false,
      returnTo: defaultReturnTo,
    };
  }

  const registrationComplete =
    value.registrationComplete === true && typeof value.email === "string";

  return {
    registrationComplete,
    email: registrationComplete ? (value.email as string) : undefined,
    authenticationRequired: value.authenticationRequired === true,
    returnTo: isSafeReturnTo(value.returnTo) ? value.returnTo : defaultReturnTo,
  };
}
