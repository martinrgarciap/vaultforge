import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ApiError } from "../api/ApiError";
import { requestJSON } from "../api/http";
import type {
  Account,
  ApiRequestOptions,
  LoginRequest,
  RegisterRequest,
} from "../api/types";
import { AuthContext } from "./AuthContext";
import {
  parseLoginResponse,
  parseRefreshResponse,
  parseRegisterResponse,
} from "./contracts";
import { readCSRFToken } from "./csrf";
import type { AuthContextValue, AuthStatus } from "./types";

interface AuthProviderProps {
  children: ReactNode;
}

const refreshPath = "/v1/auth/refresh";

export function AuthProvider({ children }: AuthProviderProps) {
  const accessTokenRef = useRef<string | null>(null);
  const refreshPromiseRef = useRef<Promise<string> | null>(null);
  const restorationStartedRef = useRef(false);

  const [account, setAccount] = useState<Account | null>(null);
  const [status, setStatus] = useState<AuthStatus>(() =>
    readCSRFToken() ? "restoring" : "unauthenticated",
  );

  const clearAuthentication = useCallback(() => {
    accessTokenRef.current = null;
    setAccount(null);
    setStatus("unauthenticated");
  }, []);

  const refreshAccessToken = useCallback((): Promise<string> => {
    const existingRefresh = refreshPromiseRef.current;

    if (existingRefresh) {
      return existingRefresh;
    }

    const refreshPromise = (async () => {
      const csrfToken = readCSRFToken();

      if (!csrfToken) {
        clearAuthentication();

        throw new ApiError(
          401,
          "refresh_unavailable",
          "Authentication could not be restored.",
        );
      }

      try {
        const rawResponse = await requestJSON<unknown>(refreshPath, {
          method: "POST",
          headers: {
            "X-CSRF-Token": csrfToken,
          },
        });

        const response = parseRefreshResponse(rawResponse);

        accessTokenRef.current = response.accessToken;
        setStatus("authenticated");

        return response.accessToken;
      } catch (error) {
        clearAuthentication();
        throw error;
      }
    })();

    refreshPromiseRef.current = refreshPromise;

    refreshPromise.then(
      () => {
        if (refreshPromiseRef.current === refreshPromise) {
          refreshPromiseRef.current = null;
        }
      },
      () => {
        if (refreshPromiseRef.current === refreshPromise) {
          refreshPromiseRef.current = null;
        }
      },
    );

    return refreshPromise;
  }, [clearAuthentication]);

  const register = useCallback(
    async (credentials: RegisterRequest): Promise<Account> => {
      const rawResponse = await requestJSON<unknown>("/v1/auth/register", {
        method: "POST",
        json: credentials,
      });

      const response = parseRegisterResponse(rawResponse);

      return response.user;
    },
    [],
  );

  const login = useCallback(
    async (credentials: LoginRequest): Promise<Account> => {
      const rawResponse = await requestJSON<unknown>("/v1/auth/login", {
        method: "POST",
        json: credentials,
      });

      const response = parseLoginResponse(rawResponse);

      accessTokenRef.current = response.accessToken;
      setAccount(response.user);
      setStatus("authenticated");

      return response.user;
    },
    [],
  );

  const authenticatedRequest = useCallback(
    async <T,>(path: string, options: ApiRequestOptions = {}): Promise<T> => {
      if (path === refreshPath) {
        throw new TypeError(
          "The refresh endpoint cannot use the authenticated request flow.",
        );
      }

      let accessToken = accessTokenRef.current;

      if (!accessToken) {
        accessToken = await refreshAccessToken();
      }

      const execute = async (token: string): Promise<T> => {
        const headers = new Headers(options.headers);

        headers.set("Authorization", `Bearer ${token}`);

        return requestJSON<T>(path, {
          ...options,
          headers,
        });
      };

      try {
        return await execute(accessToken);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 401) {
          throw error;
        }
      }

      const refreshedAccessToken = await refreshAccessToken();

      try {
        return await execute(refreshedAccessToken);
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          clearAuthentication();
        }

        throw error;
      }
    },
    [clearAuthentication, refreshAccessToken],
  );

  const logout = useCallback(async (): Promise<void> => {
    try {
      if (accessTokenRef.current || readCSRFToken()) {
        await authenticatedRequest<void>("/v1/sessions/current", {
          method: "DELETE",
        });
      }
    } finally {
      clearAuthentication();
    }
  }, [authenticatedRequest, clearAuthentication]);

  useEffect(() => {
    if (status !== "restoring" || restorationStartedRef.current) {
      return;
    }

    restorationStartedRef.current = true;

    void refreshAccessToken().catch(() => undefined);
  }, [refreshAccessToken, status]);

  const contextValue = useMemo<AuthContextValue>(
    () => ({
      status,
      account,
      register,
      login,
      logout,
      request: authenticatedRequest,
    }),
    [account, authenticatedRequest, login, logout, register, status],
  );

  return (
    <AuthContext.Provider value={contextValue}>{children}</AuthContext.Provider>
  );
}
