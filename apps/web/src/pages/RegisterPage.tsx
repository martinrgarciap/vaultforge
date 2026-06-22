import type { FormEvent } from "react";
import { useRef, useState } from "react";
import { Link, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";
import type { RegisterFieldErrors } from "../auth/validation";
import {
  hasFieldErrors,
  normalizeEmailForSubmission,
  validateRegisterFields,
} from "../auth/validation";
import { ApiErrorMessage } from "../components/ApiErrorMessage";

export function RegisterPage() {
  const navigate = useNavigate();
  const { register, status } = useAuth();

  const submittingRef = useRef(false);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<RegisterFieldErrors>({});
  const [submissionError, setSubmissionError] = useState<unknown>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submittingRef.current || status === "restoring") {
      return;
    }

    const errors = validateRegisterFields(email, password, confirmPassword);

    setFieldErrors(errors);
    setSubmissionError(null);

    if (hasFieldErrors(errors)) {
      return;
    }

    const normalizedEmail = normalizeEmailForSubmission(email);

    submittingRef.current = true;
    setIsSubmitting(true);

    try {
      await register({
        email: normalizedEmail,
        password,
      });

      setPassword("");
      setConfirmPassword("");

      navigate("/login", {
        replace: true,
        state: {
          registrationComplete: true,
          email: normalizedEmail,
        },
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
      <p className="page-kicker">Account access</p>
      <h1>Create your account</h1>
      <p className="auth-intro">
        Create a VaultForge account using synthetic data only.
      </p>

      {submissionError ? <ApiErrorMessage error={submissionError} /> : null}

      <form
        className="auth-form"
        onSubmit={handleSubmit}
        noValidate
        aria-busy={isSubmitting}
      >
        <div className="form-field">
          <label className="form-label" htmlFor="register-email">
            Email address
          </label>

          <input
            className="form-input"
            id="register-email"
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
              fieldErrors.email ? "register-email-error" : undefined
            }
          />

          {fieldErrors.email ? (
            <p className="field-error" id="register-email-error">
              {fieldErrors.email}
            </p>
          ) : null}
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="register-password">
            Password
          </label>

          <input
            className="form-input"
            id="register-password"
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
            autoComplete="new-password"
            disabled={formDisabled}
            required
            aria-invalid={fieldErrors.password ? true : undefined}
            aria-describedby={
              fieldErrors.password
                ? "register-password-help register-password-error"
                : "register-password-help"
            }
          />

          <p className="field-help" id="register-password-help">
            Use between 15 and 128 characters. Spaces are preserved.
          </p>

          {fieldErrors.password ? (
            <p className="field-error" id="register-password-error">
              {fieldErrors.password}
            </p>
          ) : null}
        </div>

        <div className="form-field">
          <label className="form-label" htmlFor="register-confirm-password">
            Confirm password
          </label>

          <input
            className="form-input"
            id="register-confirm-password"
            name="confirmPassword"
            type="password"
            value={confirmPassword}
            onChange={(event) => {
              setConfirmPassword(event.target.value);
              setFieldErrors((current) => ({
                ...current,
                confirmPassword: undefined,
              }));
              setSubmissionError(null);
            }}
            autoComplete="new-password"
            disabled={formDisabled}
            required
            aria-invalid={fieldErrors.confirmPassword ? true : undefined}
            aria-describedby={
              fieldErrors.confirmPassword
                ? "register-confirm-password-error"
                : undefined
            }
          />

          {fieldErrors.confirmPassword ? (
            <p className="field-error" id="register-confirm-password-error">
              {fieldErrors.confirmPassword}
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
              ? "Creating account..."
              : "Create account"}
        </button>
      </form>

      <p className="auth-switch">
        Already have an account?{" "}
        <Link className="text-link" to="/login">
          Sign in
        </Link>
      </p>
    </section>
  );
}
