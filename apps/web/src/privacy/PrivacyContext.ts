import { createContext } from "react";

export interface CopyValueInput {
  label: string;
  value: string;
}

export interface PrivacyContextValue {
  resetVersion: number;
  copyValue: (input: CopyValueInput) => Promise<boolean>;
}

export const PrivacyContext = createContext<PrivacyContextValue | null>(null);
