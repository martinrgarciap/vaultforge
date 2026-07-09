import type { FormEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";
import type { RegisterFieldErrors } from "../auth/validation";
import {
  hasFieldErrors,
  normalizeEmailForSubmission,
  validateRegisterFields,
} from "../auth/validation";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { PasswordStrengthResponse } from "../passwords/contracts";
import { checkPasswordStrength } from "../passwords/request";

type StrengthStatus = "idle" | "checking" | "ready" | "error";

function hasDigit(value: string) {
  return /\d/.test(value);
}

function hasSymbol(value: string) {
  return /[^A-Za-z0-9\s]/.test(value);
}

function hasUppercase(value: string) {
  return /[A-Z]/.test(value);
}

function hasLowercase(value: string) {
  return /[a-z]/.test(value);
}

function scoreClassName(score: number | null) {
  if (score === null) {
    return "strength-meter-fill strength-score-empty";
  }

  const boundedScore = Math.max(0, Math.min(score, 4));

  return `strength-meter-fill strength-score-${boundedScore}`;
}

function passwordDescriptionIds(hasPasswordError: boolean) {
  return [
    "register-password-help",
    "register-password-hints",
    "register-password-strength",
    hasPasswordError ? "register-password-error" : "",
  ]
    .filter(Boolean)
    .join(" ");
}

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
  const [strengthStatus, setStrengthStatus] = useState<StrengthStatus>("idle");
  const [strength, setStrength] = useState<PasswordStrengthResponse | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;

    if (password.length === 0) {
      return () => {
        cancelled = true;
      };
    }

    const timeoutId = window.setTimeout(() => {
      void checkPasswordStrength({
        password,
      })
        .then((response) => {
          if (cancelled) {
            return;
          }

          setStrength(response);
          setStrengthStatus("ready");
        })
        .catch(() => {
          if (cancelled) {
            return;
          }

          setStrength(null);
          setStrengthStatus("error");
        });
    }, 300);

    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [password]);

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
  const passwordHints = [
    {
      label: "15 to 128 characters",
      satisfied: password.length >= 15 && password.length <= 128,
    },
    {
      label: "At least one digit",
      satisfied: hasDigit(password),
    },
    {
      label: "At least one symbol",
      satisfied: hasSymbol(password),
    },
    {
      label: "Uppercase and lowercase letters",
      satisfied: hasUppercase(password) && hasLowercase(password),
    },
  ];

  const strengthScore =
    strengthStatus === "ready" && strength ? strength.score : null;

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
        aria-busy={formDisabled}
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
              const nextPassword = event.target.value;

              setPassword(nextPassword);

              if (nextPassword.length === 0) {
                setStrength(null);
                setStrengthStatus("idle");
              } else {
                setStrength(null);
                setStrengthStatus("checking");
              }

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
            aria-describedby={passwordDescriptionIds(
              Boolean(fieldErrors.password),
            )}
          />

          <p className="field-help" id="register-password-help">
            Use between 15 and 128 characters. Spaces are preserved. Strength
            checks are for account passwords only, not vault item secrets.
          </p>

          <ul
            className="password-hint-list"
            id="register-password-hints"
            aria-label="Password guidance"
          >
            {passwordHints.map((hint) => (
              <li
                className={
                  hint.satisfied
                    ? "password-hint password-hint-satisfied"
                    : "password-hint"
                }
                key={hint.label}
              >
                <span aria-hidden="true">{hint.satisfied ? "✓" : "○"}</span>
                {hint.label}
              </li>
            ))}
          </ul>

          <div
            className="password-strength"
            id="register-password-strength"
            role="status"
            aria-live="polite"
          >
            <div className="strength-meter" aria-hidden="true">
              <span className={scoreClassName(strengthScore)} />
            </div>

            {strengthStatus === "idle" ? (
              <p className="field-help">Enter a password to check strength.</p>
            ) : null}

            {strengthStatus === "checking" ? (
              <p className="field-help">Checking password strength...</p>
            ) : null}

            {strengthStatus === "ready" && strength ? (
              <>
                <p className="field-help">
                  Strength: <strong>{strength.label}</strong>. Estimated
                  entropy: {strength.entropyBits.toFixed(1)} bits. Crack time:
                  {strength.crackTimeEstimate}.
                </p>

                {strength.suggestions.length > 0 ? (
                  <ul className="password-strength-suggestions">
                    {strength.suggestions.map((suggestion) => (
                      <li key={suggestion}>{suggestion}</li>
                    ))}
                  </ul>
                ) : null}
              </>
            ) : null}

            {strengthStatus === "error" ? (
              <p className="field-help">
                Password strength is temporarily unavailable.
              </p>
            ) : null}
          </div>

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
