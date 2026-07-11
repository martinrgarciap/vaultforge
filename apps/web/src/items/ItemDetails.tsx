import { useEffect, useState } from "react";

import type { PasswordStrengthResponse } from "../passwords/contracts";
import { checkPasswordStrength } from "../passwords/request";
import type { ItemType, VaultItem } from "./contracts";
import { formatTimestamp } from "./display";
import { itemFieldValue, itemFormFields } from "./form";
import { ItemValue } from "./ItemValue";

interface ItemDetailsProps {
  item: VaultItem;
  onGenerateSecurePassword?: () => void;
}

const copyableFields: Record<ItemType, readonly string[]> = {
  login: ["username", "password", "website"],
  api_key: ["service", "apiKey"],
  environment_variable: ["name", "value"],
  database_connection: ["host", "port", "database", "username", "password"],
  secure_note: ["note"],
};

function scoreClassName(score: number | null) {
  if (score === null) {
    return "strength-meter-fill strength-score-empty";
  }

  const boundedScore = Math.max(0, Math.min(score, 4));

  return `strength-meter-fill strength-score-${boundedScore}`;
}

function displayLabel(label: string): string {
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function additionalValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }

  return JSON.stringify(value, null, 2);
}

function PasswordStrengthDetails({
  password,
  onGenerateSecurePassword,
}: {
  password: string;
  onGenerateSecurePassword?: () => void;
}) {
  const [result, setResult] = useState<{
    password: string;
    status: "ready" | "error";
    strength: PasswordStrengthResponse | null;
  } | null>(null);

  useEffect(() => {
    let active = true;

    if (password === "") {
      return () => {
        active = false;
      };
    }

    void checkPasswordStrength({
      password,
    })
      .then((response) => {
        if (!active) {
          return;
        }

        setResult({
          password,
          status: "ready",
          strength: response,
        });
      })
      .catch(() => {
        if (!active) {
          return;
        }

        setResult({
          password,
          status: "error",
          strength: null,
        });
      });

    return () => {
      active = false;
    };
  }, [password]);

  if (password === "") {
    return null;
  }

  const strength = result?.password === password ? result.strength : null;
  const status = result?.password === password ? result.status : "checking";
  const isWeakPassword = strength !== null && strength.score < 3;

  return (
    <div
      className={
        isWeakPassword
          ? "password-strength password-strength-warning item-detail-password-strength"
          : "password-strength item-detail-password-strength"
      }
      role="status"
      aria-live="polite"
    >
      <div className="strength-meter" aria-hidden="true">
        <span className={scoreClassName(strength?.score ?? null)} />
      </div>

      {status === "checking" ? (
        <p className="field-help">Checking password strength...</p>
      ) : null}

      {status === "ready" && strength ? (
        <p className="field-help">
          Strength: <strong>{strength.label}</strong>. Crack time:{" "}
          {strength.crackTimeEstimate}.
          {isWeakPassword
            ? " This password is weaker than recommended. Consider updating it."
            : ""}
        </p>
      ) : null}

      {status === "error" ? (
        <p className="field-help">
          Password strength is temporarily unavailable.
        </p>
      ) : null}

      {isWeakPassword && onGenerateSecurePassword ? (
        <button
          className="secondary-button item-detail-password-action"
          type="button"
          onClick={onGenerateSecurePassword}
        >
          Generate Secure Password
        </button>
      ) : null}
    </div>
  );
}

export function ItemDetails({
  item,
  onGenerateSecurePassword,
}: ItemDetailsProps) {
  const configuredFields = itemFormFields(item.type);

  const configuredKeys = new Set(configuredFields.map((field) => field.key));

  const allowedCopyFields = new Set(copyableFields[item.type]);

  const additionalFields = Object.entries(item.payload).filter(
    ([key]) => !configuredKeys.has(key),
  );

  return (
    <section className="item-details-card" aria-label="Item fields">
      <dl className="item-detail-fields">
        {configuredFields.map((field) => {
          const value = itemFieldValue(item.payload, field.key);

          return (
            <div className="item-detail-field" key={field.key}>
              <dt>{displayLabel(field.label)}</dt>

              <dd>
                <ItemValue
                  label={displayLabel(field.label)}
                  value={value}
                  sensitive={field.kind === "password"}
                  copyable={allowedCopyFields.has(field.key)}
                  multiline={field.kind === "multiline"}
                />

                {field.kind === "password" ? (
                  <PasswordStrengthDetails
                    password={value}
                    onGenerateSecurePassword={onGenerateSecurePassword}
                  />
                ) : null}
              </dd>
            </div>
          );
        })}
      </dl>

      {additionalFields.length > 0 ? (
        <details className="additional-fields">
          <summary>Additional fields</summary>

          <dl className="item-detail-fields">
            {additionalFields.map(([key, value]) => (
              <div className="item-detail-field" key={key}>
                <dt>{key}</dt>

                <dd>
                  <pre className="additional-value">
                    {additionalValue(value)}
                  </pre>
                </dd>
              </div>
            ))}
          </dl>
        </details>
      ) : null}

      <dl className="item-audit-details">
        <div>
          <dt>Created</dt>

          <dd>
            <time dateTime={item.createdAt}>
              {formatTimestamp(item.createdAt)}
            </time>
          </dd>
        </div>

        <div>
          <dt>Updated</dt>

          <dd>
            <time dateTime={item.updatedAt}>
              {formatTimestamp(item.updatedAt)}
            </time>
          </dd>
        </div>

        {item.deletedAt ? (
          <div>
            <dt>Deleted</dt>

            <dd>
              <time dateTime={item.deletedAt}>
                {formatTimestamp(item.deletedAt)}
              </time>
            </dd>
          </div>
        ) : null}
      </dl>
    </section>
  );
}
