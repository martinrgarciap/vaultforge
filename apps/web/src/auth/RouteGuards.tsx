import { Navigate, Outlet, useLocation } from "react-router";

import { LoadingState } from "../components/PageState";
import { useAuth } from "./useAuth";

function AuthenticationCheckPage() {
  return (
    <section className="page-card auth-card">
      <p className="page-kicker">Account Access</p>

      <h1>Checking Your Session</h1>

      <LoadingState message="Restoring your session..." />
    </section>
  );
}

function currentLocationPath(pathname: string, search: string): string {
  return `${pathname}${search}`;
}

export function RequireAuthentication() {
  const location = useLocation();
  const { status } = useAuth();

  if (status === "restoring") {
    return <AuthenticationCheckPage />;
  }

  if (status === "unauthenticated") {
    return (
      <Navigate
        to="/login"
        replace
        state={{
          authenticationRequired: true,
          returnTo: currentLocationPath(location.pathname, location.search),
        }}
      />
    );
  }

  return <Outlet />;
}

export function RequireGuest() {
  const { status } = useAuth();

  if (status === "restoring") {
    return <AuthenticationCheckPage />;
  }

  if (status === "authenticated") {
    return <Navigate to="/vaults" replace />;
  }

  return <Outlet />;
}

export function HomeRoute() {
  const { status } = useAuth();

  if (status === "restoring") {
    return <AuthenticationCheckPage />;
  }

  return (
    <Navigate to={status === "authenticated" ? "/vaults" : "/login"} replace />
  );
}
