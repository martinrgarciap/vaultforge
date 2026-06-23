import type {
  Account,
  ApiRequestOptions,
  LoginRequest,
  RegisterRequest,
} from "../api/types";

export type AuthStatus = "restoring" | "authenticated" | "unauthenticated";

export interface LogoutOptions {
  revokeServerSession?: boolean;
}

export interface AuthContextValue {
  status: AuthStatus;
  account: Account | null;
  register: (credentials: RegisterRequest) => Promise<Account>;
  login: (credentials: LoginRequest) => Promise<Account>;
  logout: (options?: LogoutOptions) => Promise<void>;
  request: <T = void>(path: string, options?: ApiRequestOptions) => Promise<T>;
}
