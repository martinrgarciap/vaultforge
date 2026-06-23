import type { SessionSummary } from "./contracts";
import {
  formatSessionTimestamp,
  sessionClientLabel,
  sessionUserAgentDescription,
} from "./display";

interface SessionTableProps {
  sessions: SessionSummary[];
  isActioning: boolean;
  onLogoutCurrent: (session: SessionSummary) => void;
  onRevoke: (session: SessionSummary) => void;
}

function orderedSessions(sessions: SessionSummary[]): SessionSummary[] {
  return [...sessions].sort((first, second) => {
    if (first.current !== second.current) {
      return first.current ? -1 : 1;
    }

    return Date.parse(second.createdAt) - Date.parse(first.createdAt);
  });
}

export function SessionTable({
  sessions,
  isActioning,
  onLogoutCurrent,
  onRevoke,
}: SessionTableProps) {
  return (
    <div className="session-table-container">
      <table className="session-table">
        <thead>
          <tr>
            <th scope="col">Device / Client</th>
            <th scope="col">Created</th>
            <th scope="col">Expires</th>
            <th scope="col">Status</th>
            <th scope="col">
              <span className="visually-hidden">Actions</span>
            </th>
          </tr>
        </thead>

        <tbody>
          {orderedSessions(sessions).map((session) => (
            <tr key={session.id}>
              <td data-label="Device / Client">
                <div className="session-client">
                  <span className="session-client-name">
                    {sessionClientLabel(session.userAgent)}
                  </span>

                  <span
                    className="session-user-agent"
                    title={sessionUserAgentDescription(session.userAgent)}
                  >
                    {sessionUserAgentDescription(session.userAgent)}
                  </span>
                </div>
              </td>

              <td data-label="Created">
                <time dateTime={session.createdAt}>
                  {formatSessionTimestamp(session.createdAt)}
                </time>
              </td>

              <td data-label="Expires">
                <time dateTime={session.expiresAt}>
                  {formatSessionTimestamp(session.expiresAt)}
                </time>
              </td>

              <td data-label="Status">
                {session.current ? (
                  <span className="session-status-current">
                    Current Session
                  </span>
                ) : (
                  <span className="session-status-active">Active</span>
                )}
              </td>

              <td className="session-action-cell" data-label="Action">
                {session.current ? (
                  <button
                    className="secondary-button session-action-button"
                    type="button"
                    onClick={() => {
                      onLogoutCurrent(session);
                    }}
                    disabled={isActioning}
                  >
                    Log Out This Device
                  </button>
                ) : (
                  <button
                    className="danger-outline-button session-action-button"
                    type="button"
                    onClick={() => {
                      onRevoke(session);
                    }}
                    disabled={isActioning}
                  >
                    Revoke
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
