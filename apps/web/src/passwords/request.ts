import { requestJSON } from "../api/http";
import type {
  PasswordGenerateRequest,
  PasswordGenerateResponse,
  PasswordStrengthRequest,
  PasswordStrengthResponse,
} from "./contracts";

export function generatePassword(
  request: PasswordGenerateRequest,
): Promise<PasswordGenerateResponse> {
  return requestJSON<PasswordGenerateResponse>("/v1/passwords/generate", {
    method: "POST",
    json: request,
  });
}

export function checkPasswordStrength(
  request: PasswordStrengthRequest,
): Promise<PasswordStrengthResponse> {
  return requestJSON<PasswordStrengthResponse>("/v1/passwords/strength", {
    method: "POST",
    json: request,
  });
}
