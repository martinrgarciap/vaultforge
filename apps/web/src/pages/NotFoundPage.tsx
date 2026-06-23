import { Link } from "react-router";

import { useAuth } from "../auth/useAuth";
import { LoadingState } from "../components/PageState";

export function NotFoundPage() {
  const { status } = useAuth();

  return (
    <section className="page-card">
      <p className="page-kicker">Not Found</p>

      <h1>Page Not Found</h1>

      <p>The requested VaultForge page does not exist.</p>

      {status === "restoring" ? (
        <LoadingState message="Checking your session..." />
      ) : (
        <Link
          className="text-link"
          to={status === "authenticated" ? "/vaults" : "/login"}
        >
          {status === "authenticated" ? "Return to Vaults" : "Return to Login"}
        </Link>
      )}
    </section>
  );
}
