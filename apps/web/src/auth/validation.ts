export interface LoginFieldErrors {
  email?: string;
  password?: string;
}

export interface RegisterFieldErrors extends LoginFieldErrors {
  confirmPassword?: string;
}

const maximumEmailBytes = 254;
const minimumPasswordLength = 15;
const maximumPasswordLength = 128;
const emailPattern = /^[^\s@]+@[^\s@]+$/;

export function normalizeEmailForSubmission(value: string): string {
  return value.trim().toLowerCase();
}

function emailByteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

function passwordCharacterLength(value: string): number {
  return Array.from(value.normalize("NFC")).length;
}

function validateEmail(value: string): string | undefined {
  const normalizedEmail = normalizeEmailForSubmission(value);

  if (normalizedEmail === "") {
    return "Email address is required.";
  }

  if (
    emailByteLength(normalizedEmail) > maximumEmailBytes ||
    !emailPattern.test(normalizedEmail)
  ) {
    return "Enter a valid email address.";
  }

  return undefined;
}

export function validateLoginFields(
  email: string,
  password: string,
): LoginFieldErrors {
  const errors: LoginFieldErrors = {};
  const emailError = validateEmail(email);

  if (emailError) {
    errors.email = emailError;
  }

  if (password === "") {
    errors.password = "Password is required.";
  }

  return errors;
}

export function validateRegisterFields(
  email: string,
  password: string,
  confirmPassword: string,
): RegisterFieldErrors {
  const errors: RegisterFieldErrors = {};
  const emailError = validateEmail(email);
  const passwordLength = passwordCharacterLength(password);

  if (emailError) {
    errors.email = emailError;
  }

  if (
    passwordLength < minimumPasswordLength ||
    passwordLength > maximumPasswordLength
  ) {
    errors.password = "Password must contain between 15 and 128 characters.";
  }

  if (confirmPassword === "") {
    errors.confirmPassword = "Confirm your password.";
  } else if (password.normalize("NFC") !== confirmPassword.normalize("NFC")) {
    errors.confirmPassword = "Passwords do not match.";
  }

  return errors;
}

export function hasFieldErrors(
  errors: LoginFieldErrors | RegisterFieldErrors,
): boolean {
  return Object.values(errors).some((message) => message !== undefined);
}
