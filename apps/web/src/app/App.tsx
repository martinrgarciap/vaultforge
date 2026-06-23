import { BrowserRouter } from "react-router";

import { AuthProvider } from "../auth/AuthProvider";
import { PrivacyProvider } from "../privacy/PrivacyProvider";
import { AppRoutes } from "./AppRoutes";

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <PrivacyProvider>
          <AppRoutes />
        </PrivacyProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
