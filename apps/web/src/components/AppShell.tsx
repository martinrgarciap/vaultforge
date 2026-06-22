import { NavLink, Outlet } from "react-router";

function navLinkClassName({ isActive }: { isActive: boolean }) {
  return isActive
    ? "navigation-link navigation-link-active"
    : "navigation-link";
}

export function AppShell() {
  return (
    <div className="application">
      <header className="application-header">
        <div>
          <p className="application-eyebrow">Developer secrets vault</p>
          <p className="application-title">VaultForge</p>
        </div>

        <nav className="navigation" aria-label="Primary navigation">
          <NavLink className={navLinkClassName} to="/vaults">
            Vaults
          </NavLink>

          <NavLink className={navLinkClassName} to="/sessions">
            Sessions
          </NavLink>

          <NavLink className={navLinkClassName} to="/login">
            Login
          </NavLink>

          <NavLink className={navLinkClassName} to="/register">
            Register
          </NavLink>
        </nav>
      </header>

      <div className="security-notice" role="note">
        Use synthetic data only. Browser-side encryption is not implemented.
      </div>

      <main className="application-content">
        <Outlet />
      </main>
    </div>
  );
}
