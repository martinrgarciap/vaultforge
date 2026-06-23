import type { ReactNode } from "react";
import { Link } from "react-router";

import { ApiErrorMessage } from "./ApiErrorMessage";

interface LoadingStateProps {
  message: string;
}

interface EmptyStateProps {
  children: ReactNode;
}

interface RequestErrorStateProps {
  error: unknown;
  onRetry?: () => void;
  retryLabel?: string;
}

interface ResourceNotFoundStateProps {
  title: string;
  message: string;
  to: string;
  linkLabel: string;
}

export function LoadingState({ message }: LoadingStateProps) {
  return (
    <p className="loading-message" role="status" aria-live="polite">
      {message}
    </p>
  );
}

export function EmptyState({ children }: EmptyStateProps) {
  return <div className="vault-empty">{children}</div>;
}

export function RequestErrorState({
  error,
  onRetry,
  retryLabel = "Try again",
}: RequestErrorStateProps) {
  return (
    <div className="request-error-state">
      <ApiErrorMessage error={error} />

      {onRetry ? (
        <div className="state-actions">
          <button className="secondary-button" type="button" onClick={onRetry}>
            {retryLabel}
          </button>
        </div>
      ) : null}
    </div>
  );
}

export function ResourceNotFoundState({
  title,
  message,
  to,
  linkLabel,
}: ResourceNotFoundStateProps) {
  return (
    <>
      <h1>{title}</h1>

      <EmptyState>
        <p>{message}</p>

        <Link className="text-link" to={to}>
          {linkLabel}
        </Link>
      </EmptyState>
    </>
  );
}
