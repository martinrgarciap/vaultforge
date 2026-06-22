import { ApiError } from "../api/ApiError";

interface ApiErrorMessageProps {
  error: unknown;
}

export function ApiErrorMessage({ error }: ApiErrorMessageProps) {
  const message =
    error instanceof ApiError
      ? error.message
      : "The request could not be completed. Please try again.";

  const requestId = error instanceof ApiError ? error.requestId : undefined;

  return (
    <div className="form-alert" role="alert" aria-live="assertive">
      <p>{message}</p>

      {requestId ? (
        <p className="request-id">
          Request ID: <code>{requestId}</code>
        </p>
      ) : null}
    </div>
  );
}
