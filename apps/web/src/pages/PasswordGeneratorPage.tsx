import type { FormEvent } from "react";
import { useRef, useState } from "react";

import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { PasswordStrengthResponse } from "../passwords/contracts";
import { checkPasswordStrength, generatePassword } from "../passwords/request";

const defaultLength = 24;
const minLength = 8;
const maxLength = 128;

function clampLength(value: number) {
  if (Number.isNaN(value)) {
    return minLength;
  }

  return Math.min(maxLength, Math.max(minLength, value));
}

function scoreClassName(score: number | null) {
  if (score === null) {
    return "strength-meter-fill strength-score-empty";
  }

  const boundedScore = Math.max(0, Math.min(score, 4));

  return `strength-meter-fill strength-score-${boundedScore}`;
}

export function PasswordGeneratorPage() {
  const [length, setLength] = useState(defaultLength);
  const [includeUppercase, setIncludeUppercase] = useState(true);
  const [includeLowercase, setIncludeLowercase] = useState(true);
  const [includeDigits, setIncludeDigits] = useState(true);
  const [includeSymbols, setIncludeSymbols] = useState(true);
  const [excludeChars, setExcludeChars] = useState("");
  const [generatedPassword, setGeneratedPassword] = useState("");
  const [passwordStrength, setPasswordStrength] =
    useState<PasswordStrengthResponse | null>(null);
  const [strengthUnavailable, setStrengthUnavailable] = useState(false);
  const [generationError, setGenerationError] = useState<unknown>(null);
  const [copyStatus, setCopyStatus] = useState("");
  const isGeneratingRef = useRef(false);

  const handleGenerate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (isGeneratingRef.current) {
      return;
    }

    isGeneratingRef.current = true;
    setGenerationError(null);
    setPasswordStrength(null);
    setStrengthUnavailable(false);
    setCopyStatus("");

    try {
      const response = await generatePassword({
        length,
        includeUppercase,
        includeLowercase,
        includeDigits,
        includeSymbols,
        excludeChars,
      });

      setGeneratedPassword(response.password);
      setCopyStatus("Generated password.");

      try {
        const strengthResponse = await checkPasswordStrength({
          password: response.password,
        });

        setPasswordStrength(strengthResponse);
      } catch {
        setStrengthUnavailable(true);
      }
    } catch (error) {
      setGeneratedPassword("");
      setPasswordStrength(null);
      setStrengthUnavailable(false);
      setGenerationError(error);
    } finally {
      isGeneratingRef.current = false;
    }
  };

  const handleCopy = async () => {
    if (!generatedPassword) {
      return;
    }

    try {
      await navigator.clipboard.writeText(generatedPassword);
      setCopyStatus("Copied generated password.");
    } catch {
      setCopyStatus("Copy failed. Select the password and copy it manually.");
    }
  };

  const noCharacterSetsSelected =
    !includeUppercase && !includeLowercase && !includeDigits && !includeSymbols;

  return (
    <section className="page-card password-generator-page">
      <p className="page-kicker">Password tools</p>
      <h1>Password Generator</h1>

      <p className="auth-intro">
        Generate synthetic account passwords for testing VaultForge workflows.
        Do not use this demo tool as a real password manager.
      </p>

      {generationError ? <ApiErrorMessage error={generationError} /> : null}

      <form
        className="auth-form password-generator-form"
        onSubmit={handleGenerate}
        noValidate
      >
        <div className="password-generator-primary-fields">
          <div className="form-field">
            <div className="length-label-row">
              <label className="form-label" htmlFor="password-length">
                Length ({minLength}-{maxLength} characters)
              </label>

              <input
                className="length-value-input"
                aria-label="Length value"
                type="number"
                min={minLength}
                max={maxLength}
                step="1"
                value={length}
                onChange={(event) => {
                  setLength(clampLength(Number(event.target.value)));
                  setGenerationError(null);
                  setCopyStatus("");
                }}
                required
              />
            </div>

            <input
              className="password-length-slider"
              id="password-length"
              name="length"
              type="range"
              min={minLength}
              max={maxLength}
              step="1"
              value={length}
              onChange={(event) => {
                setLength(clampLength(Number(event.target.value)));
                setGenerationError(null);
                setCopyStatus("");
              }}
              required
            />

            <p className="field-help">
              Use 16 or more characters for stronger account passwords.
            </p>
          </div>

          <div className="form-field">
            <label className="form-label" htmlFor="exclude-chars">
              Excluded characters
            </label>

            <input
              className="form-input"
              id="exclude-chars"
              name="excludeChars"
              type="text"
              placeholder="O0l1"
              value={excludeChars}
              onChange={(event) => {
                setExcludeChars(event.target.value);
                setGenerationError(null);
              }}
              autoComplete="off"
              spellCheck={false}
            />

            <p className="field-help">
              Optional. Example: exclude confusing characters like O, 0, l, and
              1.
            </p>
          </div>
        </div>

        <div className="password-generator-side-panel">
          <fieldset className="password-options">
            <legend>Include</legend>

            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={includeUppercase}
                onChange={(event) => {
                  setIncludeUppercase(event.target.checked);
                  setGenerationError(null);
                }}
              />
              Uppercase letters
            </label>

            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={includeLowercase}
                onChange={(event) => {
                  setIncludeLowercase(event.target.checked);
                  setGenerationError(null);
                }}
              />
              Lowercase letters
            </label>

            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={includeDigits}
                onChange={(event) => {
                  setIncludeDigits(event.target.checked);
                  setGenerationError(null);
                }}
              />
              Digits
            </label>

            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={includeSymbols}
                onChange={(event) => {
                  setIncludeSymbols(event.target.checked);
                  setGenerationError(null);
                }}
              />
              Symbols
            </label>
          </fieldset>

          {noCharacterSetsSelected ? (
            <p className="field-error" role="alert">
              Select at least one character set.
            </p>
          ) : null}
        </div>

        <button
          className="primary-button password-generator-submit"
          type="submit"
          disabled={noCharacterSetsSelected}
        >
          Generate password
        </button>

        <section className="password-result" aria-label="Generated password">
          <div>
            <p className="page-kicker">Generated password</p>
            <output className="generated-password">
              {generatedPassword || "Generate a password to see it here."}
            </output>

            <div className="password-strength" role="status">
              <div className="strength-meter" aria-hidden="true">
                <span
                  className={scoreClassName(passwordStrength?.score ?? null)}
                />
              </div>

              {passwordStrength ? (
                <p className="field-help">
                  Strength: <strong>{passwordStrength.label}</strong>. Crack
                  time: {passwordStrength.crackTimeEstimate}.
                </p>
              ) : (
                <p className="field-help">Strength appears after generation.</p>
              )}
            </div>

            {strengthUnavailable ? (
              <p className="field-help">
                Password strength is temporarily unavailable.
              </p>
            ) : null}

            {copyStatus ? (
              <p className="password-copy-status" role="status">
                {copyStatus}
              </p>
            ) : null}
          </div>

          <button
            className="secondary-button"
            type="button"
            onClick={handleCopy}
            disabled={!generatedPassword}
          >
            Copy
          </button>
        </section>
      </form>
    </section>
  );
}
