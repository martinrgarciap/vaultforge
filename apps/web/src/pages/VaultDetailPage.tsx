import { useParams } from "react-router";

export function VaultDetailPage() {
  const { vaultId } = useParams();

  return (
    <section className="page-card">
      <p className="page-kicker">Vault workspace</p>
      <h1>Vault details</h1>

      <p>
        Selected vault: <strong>{vaultId ?? "Unknown vault"}</strong>
      </p>

      <p>Synthetic item workflows will be implemented during Step 7G.</p>
    </section>
  );
}
