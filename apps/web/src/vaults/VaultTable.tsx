import type { KeyboardEvent } from "react";
import { useNavigate } from "react-router";

import type { Vault } from "./contracts";

interface VaultTableProps {
  vaults: Vault[];
}

function formatTimestamp(value: string): string {
  const timestamp = new Date(value);

  if (Number.isNaN(timestamp.getTime())) {
    return value;
  }

  return timestamp.toLocaleString();
}

export function VaultTable({ vaults }: VaultTableProps) {
  const navigate = useNavigate();

  const openVault = (vault: Vault) => {
    navigate(`/vaults/${encodeURIComponent(vault.id)}`);
  };

  const handleKeyDown = (
    event: KeyboardEvent<HTMLTableRowElement>,
    vault: Vault,
  ) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }

    event.preventDefault();
    openVault(vault);
  };

  return (
    <div className="vault-table-container">
      <table className="vault-table">
        <thead>
          <tr>
            <th scope="col">Name</th>
            <th scope="col">Last Updated</th>
          </tr>
        </thead>

        <tbody>
          {vaults.map((vault) => (
            <tr
              className="vault-table-row"
              key={vault.id}
              role="link"
              tabIndex={0}
              aria-label={`Open ${vault.name}`}
              onClick={() => {
                openVault(vault);
              }}
              onKeyDown={(event) => {
                handleKeyDown(event, vault);
              }}
            >
              <td data-label="Name">
                <span className="vault-table-name">{vault.name}</span>
              </td>

              <td data-label="Last Updated">
                <time dateTime={vault.updatedAt}>
                  {formatTimestamp(vault.updatedAt)}
                </time>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
