import { ApiError } from "../api/ApiError";

interface ApiErrorMessageProps {
  error: unknown;
}

export function ApiErrorMessage({ error }: ApiErrorMessageProps) {
  const message =
    error instanceof ApiError
      ? error.message
      : "The request could not be completed. Please try again.";

  return (
    <div className="form-alert" role="alert" aria-live="assertive">
      <p>{message}</p>
    </div>
  );
}
