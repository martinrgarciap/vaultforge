import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";

function navLinkClassName({ isActive }: { isActive: boolean }) {
  return isActive
    ? "navigation-link navigation-link-active"
    : "navigation-link";
}

export function AppShell() {
  const navigate = useNavigate();
  const { logout, status } = useAuth();

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
        <div>
          <p className="application-eyebrow">Developer secrets vault</p>
          <p className="application-title">VaultForge</p>
        </div>

        <nav className="navigation" aria-label="Primary navigation">
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
      </header>

      <div className="security-notice" role="note">
        Use synthetic data only. Browser-side encryption is not implemented.
        Revealed values are hidden after inactivity or when the tab is hidden.
      </div>

      <main className="application-content">
        <Outlet />
      </main>
    </div>
  );
}
