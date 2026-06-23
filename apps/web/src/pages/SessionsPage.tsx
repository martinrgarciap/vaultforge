import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";

import { ApiError } from "../api/ApiError";
import { useAuth } from "../auth/useAuth";
import { ConfirmModal } from "../components/ConfirmModal";
import {
  EmptyState,
  LoadingState,
  RequestErrorState,
} from "../components/PageState";
import type { SessionSummary } from "../sessions/contracts";
import { parseSessionListResponse } from "../sessions/contracts";
import { sessionClientLabel } from "../sessions/display";
import { sessionResourcePath, sessionsPath } from "../sessions/request";
import { SessionTable } from "../sessions/SessionTable";

type SessionConfirmation =
  | {
      kind: "revoke";
      session: SessionSummary;
    }
  | {
      kind: "current";
      session: SessionSummary;
    }
  | {
      kind: "all";
    }
  | null;

function isSessionNotFound(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 404 &&
    error.code === "session_not_found"
  );
}

export function SessionsPage() {
  const navigate = useNavigate();
  const { logout, request, status } = useAuth();
  const actionRef = useRef(false);

  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);

  const [confirmation, setConfirmation] = useState<SessionConfirmation>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [actionNotice, setActionNotice] = useState<string>();
  const [isActioning, setIsActioning] = useState(false);

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    let active = true;

    void request<unknown>(sessionsPath)
      .then(parseSessionListResponse)
      .then((response) => {
        if (!active) {
          return;
        }

        setSessions(response.sessions);
        setLoadError(null);
        setIsLoading(false);
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }

        setLoadError(error);
        setIsLoading(false);
      });

    return () => {
      active = false;
    };
  }, [reloadVersion, request, status]);

  const closeConfirmation = useCallback(() => {
    setConfirmation(null);
    setActionError(null);
  }, []);

  const refreshSessions = () => {
    setIsLoading(true);
    setLoadError(null);
    setActionNotice(undefined);
    setReloadVersion((current) => current + 1);
  };

  const beginAction = (): boolean => {
    if (actionRef.current) {
      return false;
    }

    actionRef.current = true;
    setIsActioning(true);
    setActionError(null);

    return true;
  };

  const finishAction = () => {
    actionRef.current = false;
    setIsActioning(false);
  };

  const handleRevoke = async () => {
    if (confirmation?.kind !== "revoke" || !beginAction()) {
      return;
    }

    const session = confirmation.session;
    const clientLabel = sessionClientLabel(session.userAgent);

    try {
      await request<void>(sessionResourcePath(session.id), {
        method: "DELETE",
      });

      setSessions((currentSessions) =>
        currentSessions.filter(
          (currentSession) => currentSession.id !== session.id,
        ),
      );

      setConfirmation(null);
      setActionNotice(`${clientLabel} was revoked.`);
    } catch (error) {
      if (isSessionNotFound(error)) {
        setSessions((currentSessions) =>
          currentSessions.filter(
            (currentSession) => currentSession.id !== session.id,
          ),
        );

        setConfirmation(null);

        setActionNotice(
          `${clientLabel} was already inactive and was removed from the list.`,
        );

        return;
      }

      setActionError(error);
    } finally {
      finishAction();
    }
  };

  const handleLogoutCurrent = async () => {
    if (confirmation?.kind !== "current" || !beginAction()) {
      return;
    }

    await logout().catch(() => undefined);

    navigate("/login", {
      replace: true,
    });
  };

  const handleLogoutAll = async () => {
    if (confirmation?.kind !== "all" || !beginAction()) {
      return;
    }

    try {
      await request<void>(sessionsPath, {
        method: "DELETE",
      });

      await logout({
        revokeServerSession: false,
      });

      navigate("/login", {
        replace: true,
      });
    } catch (error) {
      setActionError(error);
      finishAction();
    }
  };

  return (
    <section className="page-card vault-page">
      <div className="page-heading-row">
        <div className="page-heading-copy">
          <p className="page-kicker">Account Security</p>
          <h1>Active Sessions</h1>
        </div>

        {status === "authenticated" ? (
          <div className="page-heading-actions">
            <button
              className="danger-button"
              type="button"
              onClick={() => {
                setConfirmation({
                  kind: "all",
                });
                setActionError(null);
              }}
              disabled={isLoading || sessions.length === 0 || isActioning}
            >
              Log Out All
            </button>
          </div>
        ) : null}
      </div>

      <p className="session-page-intro">
        Review devices and clients that can currently access your VaultForge
        account.
      </p>

      {actionNotice ? (
        <div className="form-success" role="status" aria-live="polite">
          {actionNotice}
        </div>
      ) : null}

      {status === "restoring" ? (
        <LoadingState message="Restoring your session..." />
      ) : null}

      {status === "unauthenticated" ? (
        <EmptyState>
          <p>Sign in to review your active sessions.</p>

          <Link className="text-link" to="/login">
            Go to Login
          </Link>
        </EmptyState>
      ) : null}

      {status === "authenticated" && loadError ? (
        <RequestErrorState error={loadError} onRetry={refreshSessions} />
      ) : null}

      {status === "authenticated" && !loadError && isLoading ? (
        <LoadingState message="Loading sessions..." />
      ) : null}

      {status === "authenticated" &&
      !loadError &&
      !isLoading &&
      sessions.length === 0 ? (
        <EmptyState>
          <p>No active sessions were returned for this account.</p>
        </EmptyState>
      ) : null}

      {status === "authenticated" &&
      !loadError &&
      !isLoading &&
      sessions.length > 0 ? (
        <SessionTable
          sessions={sessions}
          isActioning={isActioning}
          onLogoutCurrent={(session) => {
            setConfirmation({
              kind: "current",
              session,
            });
            setActionError(null);
          }}
          onRevoke={(session) => {
            setConfirmation({
              kind: "revoke",
              session,
            });
            setActionError(null);
          }}
        />
      ) : null}

      {status === "authenticated" && confirmation?.kind === "revoke" ? (
        <ConfirmModal
          title="Revoke Session?"
          eyebrow="Account Security"
          confirmLabel="Revoke Session"
          busyLabel="Revoking Session..."
          isBusy={isActioning}
          error={actionError}
          onClose={closeConfirmation}
          onConfirm={() => {
            void handleRevoke();
          }}
        >
          <p>
            Revoke{" "}
            <strong>
              {sessionClientLabel(confirmation.session.userAgent)}
            </strong>
            ?
          </p>

          <p>
            This device or client will immediately lose access to VaultForge.
          </p>
        </ConfirmModal>
      ) : null}

      {status === "authenticated" && confirmation?.kind === "current" ? (
        <ConfirmModal
          title="Log Out This Device?"
          eyebrow="Account Security"
          confirmLabel="Log Out"
          busyLabel="Logging Out..."
          isBusy={isActioning}
          error={actionError}
          onClose={closeConfirmation}
          onConfirm={() => {
            void handleLogoutCurrent();
          }}
        >
          <p>
            Log out{" "}
            <strong>
              {sessionClientLabel(confirmation.session.userAgent)}
            </strong>
            ?
          </p>

          <p>You will need to sign in again on this device.</p>
        </ConfirmModal>
      ) : null}

      {status === "authenticated" && confirmation?.kind === "all" ? (
        <ConfirmModal
          title="Log Out All Sessions?"
          eyebrow="Account Security"
          confirmLabel="Log Out All"
          busyLabel="Logging Out All..."
          isBusy={isActioning}
          error={actionError}
          onClose={closeConfirmation}
          onConfirm={() => {
            void handleLogoutAll();
          }}
        >
          <p>
            Every signed-in device and client, including this one, will lose
            access.
          </p>

          <p>You will need to sign in again.</p>
        </ConfirmModal>
      ) : null}
    </section>
  );
}
