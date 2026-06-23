import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { ConfirmModal } from "../components/ConfirmModal";
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
    } catch (error) {
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
              className="secondary-button"
              type="button"
              onClick={refreshSessions}
              disabled={isLoading || isActioning}
            >
              Refresh
            </button>

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

      {status === "restoring" ? (
        <p className="loading-message">Restoring your session...</p>
      ) : null}

      {status === "unauthenticated" ? (
        <div className="vault-empty">
          <p>Sign in to review your active sessions.</p>

          <Link className="text-link" to="/login">
            Go to login
          </Link>
        </div>
      ) : null}

      {status === "authenticated" && loadError ? (
        <>
          <ApiErrorMessage error={loadError} />

          <button
            className="secondary-button"
            type="button"
            onClick={refreshSessions}
          >
            Try again
          </button>
        </>
      ) : null}

      {status === "authenticated" && !loadError && isLoading ? (
        <p className="loading-message">Loading sessions...</p>
      ) : null}

      {status === "authenticated" &&
      !loadError &&
      !isLoading &&
      sessions.length === 0 ? (
        <div className="vault-empty">
          <p>No active sessions were returned for this account.</p>
        </div>
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

      {confirmation?.kind === "revoke" ? (
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

      {confirmation?.kind === "current" ? (
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

      {confirmation?.kind === "all" ? (
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
