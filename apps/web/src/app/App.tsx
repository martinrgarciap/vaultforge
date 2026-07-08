import { BrowserRouter } from "react-router";

import { AuthProvider } from "../auth/AuthProvider";
import { CryptoProviderRoot } from "../crypto/CryptoProviderRoot";
import { PrivacyProvider } from "../privacy/PrivacyProvider";
import { VaultUnlockProvider } from "../vaults/VaultUnlockProvider";
import { AppRoutes } from "./AppRoutes";

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <CryptoProviderRoot>
          <VaultUnlockProvider>
            <PrivacyProvider>
              <AppRoutes />
            </PrivacyProvider>
          </VaultUnlockProvider>
        </CryptoProviderRoot>
      </AuthProvider>
    </BrowserRouter>
  );
}
