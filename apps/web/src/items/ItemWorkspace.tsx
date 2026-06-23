import { useCallback, useEffect, useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import {
  EmptyState,
  LoadingState,
  RequestErrorState,
} from "../components/PageState";
import type { ItemState, VaultItem } from "./contracts";
import { parseItemListResponse } from "./contracts";
import { ItemCreateModal } from "./ItemCreateModal";
import { ItemGroupedTables } from "./ItemGroupedTables";
import { itemCollectionPath } from "./request";

interface ItemWorkspaceProps {
  vaultId: string;
}

function appendUniqueItems(
  currentItems: VaultItem[],
  newItems: VaultItem[],
): VaultItem[] {
  const currentIDs = new Set(currentItems.map((item) => item.id));

  return [
    ...currentItems,
    ...newItems.filter((item) => !currentIDs.has(item.id)),
  ];
}

export function ItemWorkspace({ vaultId }: ItemWorkspaceProps) {
  const { request } = useAuth();
  const loadMoreRef = useRef(false);

  const [listState, setListState] = useState<ItemState>("active");
  const [items, setItems] = useState<VaultItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();

  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);

  const [loadError, setLoadError] = useState<unknown>(null);
  const [paginationError, setPaginationError] = useState<unknown>(null);

  const [reloadVersion, setReloadVersion] = useState(0);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  useEffect(() => {
    let active = true;

    void request<unknown>(itemCollectionPath(vaultId, listState))
      .then(parseItemListResponse)
      .then((response) => {
        if (!active) {
          return;
        }

        setItems(response.items);
        setNextCursor(response.nextCursor);
        setLoadError(null);
        setPaginationError(null);
        setIsLoading(false);
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }

        setLoadError(error);
        setIsLoading(false);
      });

    return () => {
      active = false;
    };
  }, [listState, reloadVersion, request, vaultId]);

  const closeCreateModal = useCallback(() => {
    setIsCreateOpen(false);
  }, []);

  const refreshItems = () => {
    setIsLoading(true);
    setItems([]);
    setNextCursor(undefined);
    setLoadError(null);
    setPaginationError(null);
    setReloadVersion((current) => current + 1);
  };

  const changeListState = (state: ItemState) => {
    if (state === listState) {
      return;
    }

    setListState(state);
    setIsLoading(true);
    setItems([]);
    setNextCursor(undefined);
    setLoadError(null);
    setPaginationError(null);
  };

  const handleLoadMore = async () => {
    if (loadMoreRef.current || !nextCursor) {
      return;
    }

    loadMoreRef.current = true;
    setIsLoadingMore(true);
    setPaginationError(null);

    try {
      const rawResponse = await request<unknown>(
        itemCollectionPath(vaultId, listState, nextCursor),
      );

      const response = parseItemListResponse(rawResponse);

      setItems((currentItems) =>
        appendUniqueItems(currentItems, response.items),
      );

      setNextCursor(response.nextCursor);
    } catch (error) {
      setPaginationError(error);
    } finally {
      loadMoreRef.current = false;
      setIsLoadingMore(false);
    }
  };

  const handleCreated = (item: VaultItem) => {
    setListState("active");

    setItems((currentItems) => [
      item,
      ...currentItems.filter((currentItem) => currentItem.id !== item.id),
    ]);

    setNextCursor(undefined);
    setLoadError(null);
    setPaginationError(null);
    setIsLoading(false);
  };

  return (
    <>
      <section className="item-workspace" aria-labelledby="vault-items-heading">
        <div className="section-heading-row">
          <div>
            <h2 id="vault-items-heading">Vault Items</h2>

            <p>Select an item to view its fields and manage it.</p>
          </div>

          <div className="button-row">
            <button
              className="primary-button"
              type="button"
              onClick={() => {
                setIsCreateOpen(true);
              }}
            >
              Create Item
            </button>

            <button
              className="secondary-button"
              type="button"
              onClick={refreshItems}
              disabled={isLoading}
            >
              Refresh
            </button>
          </div>
        </div>

        <div className="item-state-tabs" aria-label="Item state">
          <button
            className="item-state-tab"
            type="button"
            aria-pressed={listState === "active"}
            onClick={() => {
              changeListState("active");
            }}
          >
            Active Items
          </button>

          <button
            className="item-state-tab"
            type="button"
            aria-pressed={listState === "deleted"}
            onClick={() => {
              changeListState("deleted");
            }}
          >
            Deleted Items
          </button>
        </div>

        {loadError && !isLoading ? (
          <RequestErrorState error={loadError} onRetry={refreshItems} />
        ) : null}

        {!loadError && isLoading ? (
          <LoadingState message="Loading items..." />
        ) : null}

        {!loadError && !isLoading && items.length === 0 ? (
          <EmptyState>
            <p>
              {listState === "active"
                ? "No active items are stored in this vault."
                : "No deleted items are waiting for recovery."}
            </p>
          </EmptyState>
        ) : null}

        {!isLoading && items.length > 0 ? (
          <ItemGroupedTables
            vaultId={vaultId}
            items={items}
            state={listState}
          />
        ) : null}

        {paginationError ? (
          <RequestErrorState
            error={paginationError}
            onRetry={() => {
              void handleLoadMore();
            }}
            retryLabel="Retry loading more"
          />
        ) : null}

        {!paginationError && nextCursor ? (
          <div className="item-pagination">
            <button
              className="secondary-button"
              type="button"
              onClick={() => {
                void handleLoadMore();
              }}
              disabled={isLoadingMore}
            >
              {isLoadingMore ? "Loading more..." : "Load more"}
            </button>
          </div>
        ) : null}
      </section>

      {isCreateOpen ? (
        <ItemCreateModal
          vaultId={vaultId}
          onClose={closeCreateModal}
          onCreated={handleCreated}
        />
      ) : null}
    </>
  );
}
