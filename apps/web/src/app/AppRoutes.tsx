import { Route, Routes } from "react-router";

import { RequireAuthentication, RequireGuest } from "../auth/RouteGuards";
import { AppShell } from "../components/AppShell";
import { HomePage } from "../pages/HomePage";
import { ItemDetailPage } from "../pages/ItemDetailPage";
import { LoginPage } from "../pages/LoginPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { PasswordGeneratorPage } from "../pages/PasswordGeneratorPage";
import { RegisterPage } from "../pages/RegisterPage";
import { SessionsPage } from "../pages/SessionsPage";
import { VaultDetailPage } from "../pages/VaultDetailPage";
import { VaultsPage } from "../pages/VaultsPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />

        <Route path="generate" element={<PasswordGeneratorPage />} />

        <Route element={<RequireGuest />}>
          <Route path="register" element={<RegisterPage />} />

          <Route path="login" element={<LoginPage />} />
        </Route>

        <Route element={<RequireAuthentication />}>
          <Route path="vaults" element={<VaultsPage />} />

          <Route path="vaults/:vaultId" element={<VaultDetailPage />} />

          <Route
            path="vaults/:vaultId/items/:itemId"
            element={<ItemDetailPage />}
          />

          <Route path="sessions" element={<SessionsPage />} />
        </Route>

        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
