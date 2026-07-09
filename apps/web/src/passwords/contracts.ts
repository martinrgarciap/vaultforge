export interface PasswordGenerateRequest {
  length: number;
  includeUppercase: boolean;
  includeLowercase: boolean;
  includeDigits: boolean;
  includeSymbols: boolean;
  excludeChars: string;
}

export interface PasswordGenerateResponse {
  password: string;
  entropyBits: number;
}

export interface PasswordStrengthRequest {
  password: string;
}

export interface PasswordStrengthResponse {
  score: number;
  label: string;
  entropyBits: number;
  crackTimeEstimate: string;
  suggestions: string[];
}
