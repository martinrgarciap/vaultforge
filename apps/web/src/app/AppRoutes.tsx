import { Navigate, Route, Routes } from "react-router";

import { AppShell } from "../components/AppShell";
import { ItemDetailPage } from "../pages/ItemDetailPage";
import { LoginPage } from "../pages/LoginPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { RegisterPage } from "../pages/RegisterPage";
import { SessionsPage } from "../pages/SessionsPage";
import { VaultDetailPage } from "../pages/VaultDetailPage";
import { VaultsPage } from "../pages/VaultsPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/login" replace />} />

        <Route path="register" element={<RegisterPage />} />

        <Route path="login" element={<LoginPage />} />

        <Route path="vaults" element={<VaultsPage />} />

        <Route path="vaults/:vaultId" element={<VaultDetailPage />} />

        <Route
          path="vaults/:vaultId/items/:itemId"
          element={<ItemDetailPage />}
        />

        <Route path="sessions" element={<SessionsPage />} />

        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
