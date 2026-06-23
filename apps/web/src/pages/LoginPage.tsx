import type { FormEvent } from "react";
import { useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";

import { readLoginNavigationState } from "../auth/navigation";
import { useAuth } from "../auth/useAuth";
import type { LoginFieldErrors } from "../auth/validation";
import {
  hasFieldErrors,
  normalizeEmailForSubmission,
  validateLoginFields,
} from "../auth/validation";
import { ApiErrorMessage } from "../components/ApiErrorMessage";

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();

  const navigationState = readLoginNavigationState(location.state);

  const { login, status } = useAuth();

  const submittingRef = useRef(false);

  const [email, setEmail] = useState(navigationState.email ?? "");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<LoginFieldErrors>({});
  const [submissionError, setSubmissionError] = useState<unknown>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submittingRef.current || status === "restoring") {
      return;
    }

    const errors = validateLoginFields(email, password);

    setFieldErrors(errors);
    setSubmissionError(null);

    if (hasFieldErrors(errors)) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);

    try {
      await login({
        email: normalizeEmailForSubmission(email),
        password,
      });

      setPassword("");

      navigate(navigationState.returnTo, {
        replace: true,
      });
    } catch (error) {
      setSubmissionError(error);
    } finally {
      submittingRef.current = false;
      setIsSubmitting(false);
    }
  };

  const formDisabled = isSubmitting || status === "restoring";

  return (
    <section className="page-card auth-card">
      <p className="page-kicker">Account Access</p>

      <h1>Sign in</h1>

      <p className="auth-intro">Sign in to access your VaultForge workspace.</p>

      {navigationState.registrationComplete ? (
        <div className="form-success" role="status" aria-live="polite">
          Account created. Sign in to continue.
        </div>
      ) : null}

      {navigationState.authenticationRequired ? (
        <div className="form-notice" role="status" aria-live="polite">
          Your session is not active. Sign in to continue.
        </div>
      ) : null}

      {submissionError ? <ApiErrorMessage error={submissionError} /> : null}

      <form
        className="auth-form"
        onSubmit={handleSubmit}
        noValidate
        aria-busy={isSubmitting}
      >
        <div className="form-field">
          <label className="form-label" htmlFor="login-email">
            Email address
          </label>

          <input
            className="form-input"
            id="login-email"
            name="email"
            type="email"
            value={email}
            onChange={(event) => {
              setEmail(event.target.value);

              setFieldErrors((current) => ({
                ...current,
                email: undefined,
              }));

              setSubmissionError(null);
            }}
            autoComplete="email"
            autoCapitalize="none"
            spellCheck={false}
            disabled={formDisabled}
            required
            aria-invalid={fieldErrors.email ? true : undefined}
            aria-describedby={
              fieldErrors.email ? "login-email-error" : undefined
            }
          />

          {fieldErrors.email ? (
            <p className="field-error" id="login-email-error">
              {fieldErrors.email}
            </p>
          ) : null}
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="login-password">
            Password
          </label>

          <input
            className="form-input"
            id="login-password"
            name="password"
            type="password"
            value={password}
            onChange={(event) => {
              setPassword(event.target.value);

              setFieldErrors((current) => ({
                ...current,
                password: undefined,
              }));

              setSubmissionError(null);
            }}
            autoComplete="current-password"
            disabled={formDisabled}
            required
            aria-invalid={fieldErrors.password ? true : undefined}
            aria-describedby={
              fieldErrors.password ? "login-password-error" : undefined
            }
          />

          {fieldErrors.password ? (
            <p className="field-error" id="login-password-error">
              {fieldErrors.password}
            </p>
          ) : null}
        </div>

        <button
          className="primary-button"
          type="submit"
          disabled={formDisabled}
        >
          {status === "restoring"
            ? "Checking session..."
            : isSubmitting
              ? "Signing in..."
              : "Sign in"}
        </button>
      </form>

      <p className="auth-switch">
        Need an account?{" "}
        <Link className="text-link" to="/register">
          Create one
        </Link>
      </p>
    </section>
  );
}
