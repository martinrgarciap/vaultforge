import { Link } from "react-router";

export function NotFoundPage() {
  return (
    <section className="page-card">
      <p className="page-kicker">Not found</p>
      <h1>Page not found</h1>
      <p>The requested VaultForge page does not exist.</p>

      <Link className="text-link" to="/login">
        Return to login
      </Link>
    </section>
  );
}
