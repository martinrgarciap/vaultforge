import { useEffect, useRef, useState } from "react";
import { FiEye, FiEyeOff } from "react-icons/fi";

import type { PasswordStrengthResponse } from "../passwords/contracts";
import { checkPasswordStrength, generatePassword } from "../passwords/request";
import type { ItemType } from "./contracts";
import {
  type ItemFormField,
  itemFieldValue,
  itemFormFields,
  itemPayloadObject,
  updateItemPayloadField,
} from "./form";

type StrengthStatus = "idle" | "checking" | "ready" | "error";

const defaultGeneratedPasswordLength = 24;
const minimumGeneratedPasswordLength = 8;
const maximumGeneratedPasswordLength = 128;

function clampLength(value: number) {
  if (Number.isNaN(value)) {
    return minimumGeneratedPasswordLength;
  }

  return Math.min(
    maximumGeneratedPasswordLength,
    Math.max(minimumGeneratedPasswordLength, value),
  );
}

function scoreClassName(score: number | null) {
  if (score === null) {
    return "strength-meter-fill strength-score-empty";
  }

  const boundedScore = Math.max(0, Math.min(score, 4));

  return `strength-meter-fill strength-score-${boundedScore}`;
}

interface ItemPayloadFieldsProps {
  idPrefix: string;
  actionLabel: string;
  type: ItemType;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  describedBy?: string;
  fieldErrors?: Record<string, string>;
  showPlaceholders?: boolean;
  enablePasswordTools?: boolean;
  openPasswordGenerator?: boolean;
  changedFields?: ReadonlySet<string>;
}

interface PasswordToolsProps {
  field: ItemFormField;
  fieldValue: string;
  actionLabel: string;
  disabled: boolean;
  updateValue: (value: string) => void;
  openByDefault?: boolean;
}

interface StrengthResult {
  value: string;
  status: Exclude<StrengthStatus, "idle" | "checking">;
  strength: PasswordStrengthResponse | null;
}

function PasswordTools({
  field,
  fieldValue,
  actionLabel,
  disabled,
  updateValue,
  openByDefault = false,
}: PasswordToolsProps) {
  const actionIdPrefix = actionLabel.toLowerCase().replace(/\s+/g, "-");
  const [strengthResult, setStrengthResult] = useState<StrengthResult | null>(
    null,
  );
  const [isGeneratorOpen, setIsGeneratorOpen] = useState(openByDefault);
  const [length, setLength] = useState(defaultGeneratedPasswordLength);
  const [includeUppercase, setIncludeUppercase] = useState(true);
  const [includeLowercase, setIncludeLowercase] = useState(true);
  const [includeDigits, setIncludeDigits] = useState(true);
  const [includeSymbols, setIncludeSymbols] = useState(true);
  const [generatedPassword, setGeneratedPassword] = useState("");
  const [generatedStrength, setGeneratedStrength] =
    useState<PasswordStrengthResponse | null>(null);
  const [generatorError, setGeneratorError] = useState("");
  const isGeneratingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    if (fieldValue.length === 0) {
      return () => {
        cancelled = true;
      };
    }

    const timeoutId = window.setTimeout(() => {
      void checkPasswordStrength({
        password: fieldValue,
      })
        .then((response) => {
          if (cancelled) {
            return;
          }

          setStrengthResult({
            value: fieldValue,
            status: "ready",
            strength: response,
          });
        })
        .catch(() => {
          if (cancelled) {
            return;
          }

          setStrengthResult({
            value: fieldValue,
            status: "error",
            strength: null,
          });
        });
    }, 300);

    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [fieldValue]);

  const noCharacterSetsSelected =
    !includeUppercase && !includeLowercase && !includeDigits && !includeSymbols;
  const strengthStatus: StrengthStatus =
    fieldValue.length === 0
      ? "idle"
      : strengthResult?.value === fieldValue
        ? strengthResult.status
        : "checking";
  const strength =
    strengthResult?.value === fieldValue ? strengthResult.strength : null;
  const isWeakPassword =
    strengthStatus === "ready" && strength !== null && strength.score < 3;

  const handleGenerate = async () => {
    if (isGeneratingRef.current || noCharacterSetsSelected) {
      return;
    }

    isGeneratingRef.current = true;
    setGeneratorError("");
    setGeneratedPassword("");
    setGeneratedStrength(null);

    try {
      const response = await generatePassword({
        length,
        includeUppercase,
        includeLowercase,
        includeDigits,
        includeSymbols,
        excludeChars: "",
      });

      setGeneratedPassword(response.password);

      try {
        setGeneratedStrength(
          await checkPasswordStrength({
            password: response.password,
          }),
        );
      } catch {
        setGeneratedStrength(null);
      }
    } catch {
      setGeneratorError("Password generation is temporarily unavailable.");
    } finally {
      isGeneratingRef.current = false;
    }
  };

  return (
    <div className="password-field-tools">
      {!isGeneratorOpen ? (
        <button
          className="secondary-button password-generate-toggle"
          type="button"
          onClick={() => {
            setIsGeneratorOpen(true);
            setGeneratorError("");
          }}
          disabled={disabled}
        >
          Generate Stronger Password
        </button>
      ) : null}

      <div
        className={
          isWeakPassword
            ? "password-strength password-strength-warning"
            : "password-strength"
        }
        role="status"
        aria-live="polite"
      >
        <div className="strength-meter" aria-hidden="true">
          <span className={scoreClassName(strength?.score ?? null)} />
        </div>

        {strengthStatus === "idle" ? (
          <p className="field-help">Enter a password to check its strength.</p>
        ) : null}

        {strengthStatus === "checking" ? (
          <p className="field-help">Checking password strength...</p>
        ) : null}

        {strengthStatus === "ready" && strength ? (
          <p className="field-help">
            Strength: <strong>{strength.label}</strong>. Crack time:{" "}
            {strength.crackTimeEstimate}.
            {isWeakPassword
              ? " This password is weaker than recommended. Consider updating it."
              : ""}
          </p>
        ) : null}

        {strengthStatus === "error" ? (
          <p className="field-help">
            Password strength is temporarily unavailable.
          </p>
        ) : null}
      </div>

      {isGeneratorOpen ? (
        <div className="inline-password-generator">
          <div className="inline-password-generator-heading">
            <p className="page-kicker">Password tools</p>
            <h3>Generate Secure Password</h3>
          </div>

          {generatorError ? (
            <p className="field-error" role="alert">
              {generatorError}
            </p>
          ) : null}

          <div className="form-field">
            <div className="length-label-row">
              <label
                className="form-label"
                htmlFor={`${actionIdPrefix}-${field.key}-generated-length`}
              >
                Length
              </label>

              <input
                className="length-value-input"
                id={`${actionIdPrefix}-${field.key}-generated-length`}
                type="number"
                min={minimumGeneratedPasswordLength}
                max={maximumGeneratedPasswordLength}
                step="1"
                value={length}
                onChange={(event) => {
                  setLength(clampLength(Number(event.target.value)));
                  setGeneratorError("");
                }}
              />
            </div>

            <input
              className="password-length-slider"
              aria-label="Generated password length"
              type="range"
              min={minimumGeneratedPasswordLength}
              max={maximumGeneratedPasswordLength}
              step="1"
              value={length}
              onChange={(event) => {
                setLength(clampLength(Number(event.target.value)));
                setGeneratorError("");
              }}
            />
          </div>

          <fieldset className="password-options">
            <legend>Include</legend>

            <label className="checkbox-field">
              <input
                type="checkbox"
                checked={includeUppercase}
                onChange={(event) => {
                  setIncludeUppercase(event.target.checked);
                  setGeneratorError("");
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
                  setGeneratorError("");
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
                  setGeneratorError("");
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
                  setGeneratorError("");
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

          <button
            className="secondary-button"
            type="button"
            onClick={() => {
              void handleGenerate();
            }}
            disabled={disabled || noCharacterSetsSelected}
          >
            Generate password
          </button>

          <section
            className="password-result inline-password-result"
            aria-label="Generated password"
          >
            <div>
              <p className="page-kicker">Generated password</p>
              <output className="generated-password">
                {generatedPassword || "Generate a password to preview it."}
              </output>

              <div className="password-strength" role="status">
                <div className="strength-meter" aria-hidden="true">
                  <span
                    className={scoreClassName(generatedStrength?.score ?? null)}
                  />
                </div>

                {generatedStrength ? (
                  <p className="field-help">
                    Strength: <strong>{generatedStrength.label}</strong>. Crack
                    time: {generatedStrength.crackTimeEstimate}.
                  </p>
                ) : (
                  <p className="field-help">
                    Strength appears after generation.
                  </p>
                )}
              </div>
            </div>
          </section>

          <div className="inline-password-generator-actions">
            <button
              className="secondary-button"
              type="button"
              onClick={() => {
                setIsGeneratorOpen(false);
              }}
            >
              Cancel
            </button>

            <button
              className="primary-button"
              type="button"
              onClick={() => {
                updateValue(generatedPassword);
                setIsGeneratorOpen(false);
              }}
              disabled={!generatedPassword || disabled}
            >
              Use this password
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function ItemPayloadFields({
  idPrefix,
  actionLabel,
  type,
  value,
  onChange,
  disabled = false,
  describedBy,
  fieldErrors = {},
  showPlaceholders = true,
  enablePasswordTools = false,
  openPasswordGenerator = false,
  changedFields,
}: ItemPayloadFieldsProps) {
  const [revealedFields, setRevealedFields] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const payload = itemPayloadObject(type, value);
  const fields = itemFormFields(type);
  const passwordToolsField = fields.find(
    (field) => field.kind === "password" && field.key === "password",
  );

  return (
    <div className="item-field-grid">
      {fields.map((field) => {
        const id = `${idPrefix}-${field.key}`;
        const errorId = `${id}-error`;
        const fieldValue = itemFieldValue(payload, field.key);
        const fieldError = fieldErrors[field.key];
        const fieldDescribedBy = [describedBy, fieldError ? errorId : ""]
          .filter(Boolean)
          .join(" ");
        const isPassword = field.kind === "password";
        const isRevealed = revealedFields.has(field.key);

        const updateValue = (nextValue: string) => {
          onChange(updateItemPayloadField(type, value, field.key, nextValue));
        };

        const toggleReveal = () => {
          setRevealedFields((current) => {
            const next = new Set(current);

            if (next.has(field.key)) {
              next.delete(field.key);
            } else {
              next.add(field.key);
            }

            return next;
          });
        };

        return (
          <div
            className={["form-field", field.wide ? "item-field-wide" : ""]
              .filter(Boolean)
              .join(" ")}
            key={field.key}
          >
            <label className="form-label" htmlFor={id}>
              {field.label}
              {field.required ? (
                <span className="required-marker" aria-hidden="true">
                  *
                </span>
              ) : null}
              {changedFields?.has(field.key) ? (
                <span className="changed-marker">Edited</span>
              ) : null}
            </label>

            {field.kind === "multiline" ? (
              <textarea
                className="form-input item-textarea"
                id={id}
                value={fieldValue}
                onChange={(event) => {
                  updateValue(event.target.value);
                }}
                disabled={disabled}
                placeholder={showPlaceholders ? field.placeholder : undefined}
                required={field.required}
                rows={6}
                aria-label={field.label}
                aria-invalid={fieldError ? true : undefined}
                aria-describedby={fieldDescribedBy || undefined}
              />
            ) : (
              <div className={isPassword ? "secret-input" : undefined}>
                <input
                  className="form-input"
                  id={id}
                  type={
                    isPassword
                      ? isRevealed
                        ? "text"
                        : "password"
                      : field.kind === "url"
                        ? "url"
                        : field.kind === "number"
                          ? "number"
                          : "text"
                  }
                  value={fieldValue}
                  onChange={(event) => {
                    updateValue(event.target.value);
                  }}
                  disabled={disabled}
                  placeholder={showPlaceholders ? field.placeholder : undefined}
                  required={field.required}
                  step={field.kind === "number" ? "1" : undefined}
                  autoComplete="off"
                  aria-label={field.label}
                  aria-invalid={fieldError ? true : undefined}
                  aria-describedby={fieldDescribedBy || undefined}
                />

                {isPassword ? (
                  <button
                    className="secret-toggle-button"
                    type="button"
                    onClick={toggleReveal}
                    disabled={disabled}
                    aria-label={`${isRevealed ? "Hide" : "Show"} ${actionLabel.toLowerCase()} ${field.label}`}
                    aria-pressed={isRevealed}
                  >
                    {isRevealed ? <FiEyeOff /> : <FiEye />}
                  </button>
                ) : null}
              </div>
            )}

            {fieldError ? (
              <p className="field-error" id={errorId}>
                {fieldError}
              </p>
            ) : null}
          </div>
        );
      })}

      {enablePasswordTools && passwordToolsField ? (
        <div className="form-field item-field-wide item-password-tools-field">
          <PasswordTools
            field={passwordToolsField}
            fieldValue={itemFieldValue(payload, passwordToolsField.key)}
            actionLabel={actionLabel}
            disabled={disabled}
            openByDefault={openPasswordGenerator}
            updateValue={(nextValue) => {
              onChange(
                updateItemPayloadField(
                  type,
                  value,
                  passwordToolsField.key,
                  nextValue,
                ),
              );
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
