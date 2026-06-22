export interface Account {
  id: string;
  email: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface ApiErrorDetails {
  code: string;
  message: string;
  request_id?: string;
}

export interface ApiErrorEnvelope {
  error: ApiErrorDetails;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  user: Account;
  tokenType: string;
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshTokenExpiresAt: string;
}

export interface RefreshResponse {
  tokenType: string;
  accessToken: string;
  accessTokenExpiresAt: string;
  refreshTokenExpiresAt: string;
}

export type ApiRequestOptions = Omit<RequestInit, "body" | "credentials"> & {
  json?: unknown;
};
