import type { ItemType, VaultItem } from "./contracts";
import { formatTimestamp } from "./display";
import { itemFieldValue, itemFormFields } from "./form";
import { ItemValue } from "./ItemValue";

interface ItemDetailsProps {
  item: VaultItem;
}

const copyableFields: Record<ItemType, readonly string[]> = {
  login: ["username", "password", "website"],
  api_key: ["service", "apiKey"],
  environment_variable: ["name", "value"],
  database_connection: ["host", "port", "database", "username", "password"],
  secure_note: ["note"],
};

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

export function ItemDetails({ item }: ItemDetailsProps) {
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
