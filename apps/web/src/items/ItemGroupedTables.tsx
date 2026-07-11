import type { KeyboardEvent, ReactNode } from "react";
import { useNavigate } from "react-router";

import type { ItemState, ItemType, VaultItem } from "./contracts";
import { formatTimestamp, itemDisplayName } from "./display";
import { ItemValue } from "./ItemValue";

interface ItemGroupedTablesProps {
  vaultId: string;
  items: VaultItem[];
  state: ItemState;
}

interface ItemColumn {
  heading: string;
  render: (item: VaultItem) => ReactNode;
}

interface ItemTableConfiguration {
  heading: string;
  columns: readonly ItemColumn[];
}

const itemTypeOrder: readonly ItemType[] = [
  "login",
  "api_key",
  "environment_variable",
  "database_connection",
  "secure_note",
];

function payloadValue(item: VaultItem, key: string): string {
  const value = item.payload[key];

  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }

  return "";
}

function compactValue(
  item: VaultItem,
  key: string,
  label: string,
  options: {
    sensitive?: boolean;
    copyable?: boolean;
    link?: boolean;
  } = {},
) {
  return (
    <ItemValue
      label={label}
      value={payloadValue(item, key)}
      sensitive={options.sensitive}
      copyable={options.copyable}
      link={options.link}
      compact
    />
  );
}

const configurations: Record<ItemType, ItemTableConfiguration> = {
  login: {
    heading: "Logins",
    columns: [
      {
        heading: "Title",
        render: (item) => (
          <span className="grouped-item-primary">{itemDisplayName(item)}</span>
        ),
      },
      {
        heading: "Website",
        render: (item) =>
          compactValue(item, "website", "Website", {
            copyable: true,
            link: true,
          }),
      },
      {
        heading: "Username",
        render: (item) =>
          compactValue(item, "username", "Username", {
            copyable: true,
          }),
      },
      {
        heading: "Password",
        render: (item) =>
          compactValue(item, "password", "Password", {
            sensitive: true,
            copyable: true,
          }),
      },
    ],
  },

  api_key: {
    heading: "API Keys",
    columns: [
      {
        heading: "Name",
        render: (item) => (
          <span className="grouped-item-primary">{itemDisplayName(item)}</span>
        ),
      },
      {
        heading: "Service",
        render: (item) =>
          compactValue(item, "service", "Service", {
            copyable: true,
          }),
      },
      {
        heading: "API Key",
        render: (item) =>
          compactValue(item, "apiKey", "API Key", {
            sensitive: true,
            copyable: true,
          }),
      },
    ],
  },

  environment_variable: {
    heading: "Environment Variables",
    columns: [
      {
        heading: "Variable",
        render: (item) =>
          compactValue(item, "name", "Variable", {
            copyable: true,
          }),
      },
      {
        heading: "Value",
        render: (item) =>
          compactValue(item, "value", "Value", {
            sensitive: true,
            copyable: true,
          }),
      },
    ],
  },

  database_connection: {
    heading: "Database Connections",
    columns: [
      {
        heading: "Name",
        render: (item) => (
          <span className="grouped-item-primary">{itemDisplayName(item)}</span>
        ),
      },
      {
        heading: "Host",
        render: (item) =>
          compactValue(item, "host", "Host", {
            copyable: true,
          }),
      },
      {
        heading: "Port",
        render: (item) =>
          compactValue(item, "port", "Port", {
            copyable: true,
          }),
      },
      {
        heading: "Database",
        render: (item) =>
          compactValue(item, "database", "Database", {
            copyable: true,
          }),
      },
      {
        heading: "Username",
        render: (item) =>
          compactValue(item, "username", "Username", {
            copyable: true,
          }),
      },
      {
        heading: "Password",
        render: (item) =>
          compactValue(item, "password", "Password", {
            sensitive: true,
            copyable: true,
          }),
      },
    ],
  },

  secure_note: {
    heading: "Secure Notes",
    columns: [
      {
        heading: "Title",
        render: (item) => (
          <span className="grouped-item-primary">{itemDisplayName(item)}</span>
        ),
      },
      {
        heading: "Last Updated",
        render: (item) => (
          <time dateTime={item.updatedAt}>
            {formatTimestamp(item.updatedAt)}
          </time>
        ),
      },
    ],
  },
};

function itemDetailsPath(
  vaultId: string,
  itemId: string,
  state: ItemState,
): string {
  const query = new URLSearchParams({
    state,
  });

  return (
    `/vaults/${encodeURIComponent(vaultId)}` +
    `/items/${encodeURIComponent(itemId)}` +
    `?${query.toString()}`
  );
}

export function ItemGroupedTables({
  vaultId,
  items,
  state,
}: ItemGroupedTablesProps) {
  const navigate = useNavigate();

  const openItem = (item: VaultItem) => {
    navigate(itemDetailsPath(vaultId, item.id, state));
  };

  const handleRowKeyDown = (
    event: KeyboardEvent<HTMLTableRowElement>,
    item: VaultItem,
  ) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }

    event.preventDefault();
    openItem(item);
  };

  return (
    <div className="grouped-item-tables">
      {itemTypeOrder.map((type) => {
        const typedItems = items.filter((item) => item.type === type);

        if (typedItems.length === 0) {
          return null;
        }

        const configuration = configurations[type];

        const headingId = `item-group-${type}`;

        return (
          <section
            className="grouped-item-section"
            aria-labelledby={headingId}
            key={type}
          >
            <h3 className="grouped-item-heading" id={headingId}>
              {configuration.heading}
            </h3>

            <div className="grouped-item-table-container">
              <table className="grouped-item-table">
                <thead>
                  <tr>
                    {configuration.columns.map((column) => (
                      <th scope="col" key={column.heading}>
                        {column.heading}
                      </th>
                    ))}
                  </tr>
                </thead>

                <tbody>
                  {typedItems.map((item) => {
                    const itemName = itemDisplayName(item);

                    return (
                      <tr
                        className="grouped-item-row"
                        key={item.id}
                        role="link"
                        tabIndex={0}
                        aria-label={`Open ${itemName}`}
                        onClick={() => {
                          openItem(item);
                        }}
                        onKeyDown={(event) => {
                          handleRowKeyDown(event, item);
                        }}
                      >
                        {configuration.columns.map((column) => (
                          <td data-label={column.heading} key={column.heading}>
                            {column.render(item)}
                          </td>
                        ))}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>
        );
      })}
    </div>
  );
}
