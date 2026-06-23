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

interface ItemListCacheEntry {
  items: VaultItem[];
  nextCursor?: string;
  status: "idle" | "loading" | "loaded" | "error";
  error?: unknown;
}

type ItemListCache = Record<ItemState, ItemListCacheEntry>;

function createEmptyListCacheEntry(): ItemListCacheEntry {
  return {
    items: [],
    status: "loading",
  };
}

function createInitialListCache(): ItemListCache {
  return {
    active: createEmptyListCacheEntry(),
    deleted: createEmptyListCacheEntry(),
  };
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
  const [listCache, setListCache] = useState<ItemListCache>(
    createInitialListCache,
  );

  const [isLoadingMore, setIsLoadingMore] = useState(false);

  const [paginationError, setPaginationError] = useState<unknown>(null);

  const [reloadVersion, setReloadVersion] = useState(0);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const currentList = listCache[listState];
  const items = currentList.items;
  const nextCursor = currentList.nextCursor;
  const isLoading = currentList.status === "loading";
  const loadError = currentList.status === "error" ? currentList.error : null;
  const hasLoaded = currentList.status === "loaded";

  useEffect(() => {
    let active = true;

    const loadState = (state: ItemState) => {
      void request<unknown>(itemCollectionPath(vaultId, state))
        .then(parseItemListResponse)
        .then((response) => {
          if (!active) {
            return;
          }

          setListCache((currentCache) => ({
            ...currentCache,
            [state]: {
              items: response.items,
              nextCursor: response.nextCursor,
              status: "loaded",
            },
          }));
          setPaginationError(null);
        })
        .catch((error: unknown) => {
          if (!active) {
            return;
          }

          setListCache((currentCache) => ({
            ...currentCache,
            [state]: {
              ...currentCache[state],
              status: "error",
              error,
            },
          }));
        });
    };

    loadState("active");
    loadState("deleted");

    return () => {
      active = false;
    };
  }, [reloadVersion, request, vaultId]);

  const closeCreateModal = useCallback(() => {
    setIsCreateOpen(false);
  }, []);

  const refreshItems = () => {
    setPaginationError(null);
    setReloadVersion((current) => current + 1);
  };

  const changeListState = (state: ItemState) => {
    if (state === listState) {
      return;
    }

    setListState(state);
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

      setListCache((currentCache) => ({
        ...currentCache,
        [listState]: {
          items: appendUniqueItems(
            currentCache[listState].items,
            response.items,
          ),
          nextCursor: response.nextCursor,
          status: "loaded",
        },
      }));
    } catch (error) {
      setPaginationError(error);
    } finally {
      loadMoreRef.current = false;
      setIsLoadingMore(false);
    }
  };

  const handleCreated = (item: VaultItem) => {
    setListState("active");
    setListCache((currentCache) => ({
      ...currentCache,
      active: {
        items: [
          item,
          ...currentCache.active.items.filter(
            (currentItem) => currentItem.id !== item.id,
          ),
        ],
        status: "loaded",
      },
    }));
    setPaginationError(null);
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

        {!loadError && isLoading && !hasLoaded ? (
          <LoadingState message="Loading items..." />
        ) : null}

        {!loadError && !isLoading && hasLoaded && items.length === 0 ? (
          <EmptyState>
            <p>
              {listState === "active"
                ? "No active items are stored in this vault."
                : "No deleted items are waiting for recovery."}
            </p>
          </EmptyState>
        ) : null}

        {hasLoaded && items.length > 0 ? (
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
