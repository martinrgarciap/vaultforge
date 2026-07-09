import type { FormEvent } from "react";
import { useState } from "react";

import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { generatePassword } from "../passwords/request";

const defaultLength = 24;

export function PasswordGeneratorPage() {
  const [length, setLength] = useState(defaultLength);
  const [includeUppercase, setIncludeUppercase] = useState(true);
  const [includeLowercase, setIncludeLowercase] = useState(true);
  const [includeDigits, setIncludeDigits] = useState(true);
  const [includeSymbols, setIncludeSymbols] = useState(true);
  const [excludeChars, setExcludeChars] = useState("");
  const [generatedPassword, setGeneratedPassword] = useState("");
  const [entropyBits, setEntropyBits] = useState<number | null>(null);
  const [generationError, setGenerationError] = useState<unknown>(null);
  const [copyStatus, setCopyStatus] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);

  const handleGenerate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (isGenerating) {
      return;
    }

    setIsGenerating(true);
    setGenerationError(null);
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
      setEntropyBits(response.entropyBits);
    } catch (error) {
      setGeneratedPassword("");
      setEntropyBits(null);
      setGenerationError(error);
    } finally {
      setIsGenerating(false);
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
        className="auth-form"
        onSubmit={handleGenerate}
        noValidate
        aria-busy={isGenerating}
      >
        <div className="form-field">
          <label className="form-label" htmlFor="password-length">
            Length
          </label>

          <input
            className="form-input"
            id="password-length"
            name="length"
            type="number"
            min="8"
            max="128"
            value={length}
            onChange={(event) => {
              setLength(Number(event.target.value));
              setGenerationError(null);
              setCopyStatus("");
            }}
            disabled={isGenerating}
            required
          />

          <p className="field-help">
            Use 16 or more characters for stronger account passwords.
          </p>
        </div>

        <fieldset className="password-options">
          <legend>Character sets</legend>

          <label className="checkbox-field">
            <input
              type="checkbox"
              checked={includeUppercase}
              onChange={(event) => {
                setIncludeUppercase(event.target.checked);
                setGenerationError(null);
              }}
              disabled={isGenerating}
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
              disabled={isGenerating}
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
              disabled={isGenerating}
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
              disabled={isGenerating}
            />
            Symbols
          </label>
        </fieldset>

        {noCharacterSetsSelected ? (
          <p className="field-error" role="alert">
            Select at least one character set.
          </p>
        ) : null}

        <div className="form-field">
          <label className="form-label" htmlFor="exclude-chars">
            Excluded characters
          </label>

          <input
            className="form-input"
            id="exclude-chars"
            name="excludeChars"
            type="text"
            value={excludeChars}
            onChange={(event) => {
              setExcludeChars(event.target.value);
              setGenerationError(null);
            }}
            autoComplete="off"
            spellCheck={false}
            disabled={isGenerating}
          />

          <p className="field-help">
            Optional. Example: exclude confusing characters like O, 0, l, and 1.
          </p>
        </div>

        <button
          className="primary-button"
          type="submit"
          disabled={isGenerating || noCharacterSetsSelected}
        >
          {isGenerating ? "Generating..." : "Generate password"}
        </button>
      </form>

      {generatedPassword ? (
        <section className="password-result" aria-label="Generated password">
          <div>
            <p className="page-kicker">Generated password</p>
            <output className="generated-password">{generatedPassword}</output>

            {entropyBits !== null ? (
              <p className="field-help">
                Estimated entropy: {entropyBits.toFixed(1)} bits
              </p>
            ) : null}
          </div>

          <button
            className="secondary-button"
            type="button"
            onClick={handleCopy}
          >
            Copy
          </button>
        </section>
      ) : null}

      {copyStatus ? (
        <p className="form-success" role="status">
          {copyStatus}
        </p>
      ) : null}
    </section>
  );
}
