import type { Account, ApiRequestOptions, LoginRequest } from "../api/types";

export type AuthStatus = "restoring" | "authenticated" | "unauthenticated";

export interface AuthContextValue {
  status: AuthStatus;
  account: Account | null;
  login: (credentials: LoginRequest) => Promise<Account>;
  logout: () => Promise<void>;
  request: <T = void>(path: string, options?: ApiRequestOptions) => Promise<T>;
}
