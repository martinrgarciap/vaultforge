import { useContext } from "react";

import { PrivacyContext } from "./PrivacyContext";
import type { PrivacyContextValue } from "./PrivacyContext";

export function usePrivacy(): PrivacyContextValue {
  const context = useContext(PrivacyContext);

  if (!context) {
    throw new Error("usePrivacy must be used within a PrivacyProvider.");
  }

  return context;
}
