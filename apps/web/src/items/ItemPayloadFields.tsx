import type { ItemType } from "./contracts";
import {
  itemFieldValue,
  itemFormFields,
  itemPayloadObject,
  updateItemPayloadField,
} from "./form";

interface ItemPayloadFieldsProps {
  idPrefix: string;
  labelPrefix: string;
  type: ItemType;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  describedBy?: string;
}

export function ItemPayloadFields({
  idPrefix,
  labelPrefix,
  type,
  value,
  onChange,
  disabled = false,
  describedBy,
}: ItemPayloadFieldsProps) {
  const payload = itemPayloadObject(type, value);

  return (
    <div className="item-field-grid">
      {itemFormFields(type).map((field) => {
        const id = `${idPrefix}-${field.key}`;
        const fieldValue = itemFieldValue(payload, field.key);

        const updateValue = (nextValue: string) => {
          onChange(updateItemPayloadField(type, value, field.key, nextValue));
        };

        return (
          <div
            className={field.wide ? "form-field item-field-wide" : "form-field"}
            key={field.key}
          >
            <label className="form-label" htmlFor={id}>
              {labelPrefix} {field.label}
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
                placeholder={field.placeholder}
                rows={6}
                aria-describedby={describedBy}
              />
            ) : (
              <input
                className="form-input"
                id={id}
                type={
                  field.kind === "password"
                    ? "password"
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
                placeholder={field.placeholder}
                step={field.kind === "number" ? "1" : undefined}
                autoComplete="off"
                aria-describedby={describedBy}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
