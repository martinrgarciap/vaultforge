import { useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";

function navLinkClassName({ isActive }: { isActive: boolean }) {
  return isActive
    ? "navigation-link navigation-link-active"
    : "navigation-link";
}

export function AppShell() {
  const navigate = useNavigate();
  const { account, logout, status } = useAuth();

  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const handleLogout = async () => {
    if (isLoggingOut) {
      return;
    }

    setIsLoggingOut(true);

    await logout().catch(() => undefined);

    navigate("/login", {
      replace: true,
    });

    setIsLoggingOut(false);
  };

  return (
    <div className="application">
      <header className="application-header">
        <Link
          className="application-brand"
          to="/"
          aria-label="Go to VaultForge home"
        >
          <span className="application-eyebrow">Developer secrets vault</span>
          <span className="application-title">VaultForge</span>
        </Link>

        <div className="application-header-actions">
          {status === "authenticated" && account ? (
            <p className="account-greeting">Hello, {account.email}</p>
          ) : null}

          <nav className="navigation" aria-label="Primary navigation">
            <NavLink className={navLinkClassName} to="/">
              Home
            </NavLink>

            {status === "authenticated" ? (
              <>
                <NavLink className={navLinkClassName} to="/vaults">
                  Vaults
                </NavLink>

                <NavLink className={navLinkClassName} to="/sessions">
                  Sessions
                </NavLink>

                <button
                  className="navigation-link navigation-button"
                  type="button"
                  onClick={() => {
                    void handleLogout();
                  }}
                  disabled={isLoggingOut}
                >
                  {isLoggingOut ? "Logging out..." : "Log out"}
                </button>
              </>
            ) : null}

            {status === "unauthenticated" ? (
              <>
                <NavLink className={navLinkClassName} to="/login">
                  Login
                </NavLink>

                <NavLink className={navLinkClassName} to="/register">
                  Register
                </NavLink>
              </>
            ) : null}
          </nav>
        </div>
      </header>

      <div className="security-notice" role="note">
        Use synthetic data only. Vault item payloads are encrypted in the
        browser before they are sent to the API. Unlocked vault keys and
        revealed values are cleared after inactivity or when the tab is hidden.
      </div>

      <main className="application-content">
        <Outlet />
      </main>
    </div>
  );
}
