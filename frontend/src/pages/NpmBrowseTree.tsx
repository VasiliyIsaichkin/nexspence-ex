import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  ChevronDown,
  ChevronRight,
  FileArchive,
  Package,
  Tag,
} from "lucide-react";
import { apiClient, nexusApi } from "@/api/client";
import type { Component } from "./browseTypes";
import { useBrowseSession } from "./useDockerBranch";

interface Item {
  name: string;
  version?: string;
  componentId?: string;
  repository?: string;
}
interface Page {
  items: Item[];
  nextCursor?: string;
}
const row: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 8,
  width: "100%",
  padding: "9px 12px",
  border: 0,
  background: "transparent",
  color: "var(--holo-text)",
  textAlign: "left",
  cursor: "pointer",
  fontSize: 13,
};

function useBranch(repository: string, name = "", enabled = true) {
  const session = useBrowseSession();
  return useInfiniteQuery({
    queryKey: ["npmBrowseTree", repository, session, name],
    initialPageParam: "",
    enabled,
    retry: false,
    queryFn: ({ pageParam }) =>
      apiClient
        .get(
          `/api/v1/browse/repositories/${encodeURIComponent(repository)}/npm-tree`,
          {
            params: {
              name,
              limit: 100,
              ...(pageParam ? { cursor: pageParam } : {}),
            },
          },
        )
        .then((r) => r.data as Page),
    getNextPageParam: (page: Page) => page.nextCursor || undefined,
  });
}
function useDetail(id: string, enabled: boolean) {
  const session = useBrowseSession();
  return useQuery({
    queryKey: ["npmComponentDetail", id, session],
    enabled: enabled && !!id,
    retry: false,
    queryFn: () => nexusApi.getComponent(id).then((r) => r.data as Component),
  });
}
function BranchStatus({ branch }: { branch: ReturnType<typeof useBranch> }) {
  return (
    <>
      {branch.isLoading && <p role="status">Загрузка…</p>}
      {branch.isError && (
        <p role="alert">
          Не удалось загрузить дерево.{" "}
          <button onClick={() => void branch.refetch()}>Повторить</button>
        </p>
      )}
      {branch.hasNextPage && (
        <button
          style={row}
          disabled={branch.isFetchingNextPage}
          onClick={() => void branch.fetchNextPage()}
        >
          Загрузить ещё
        </button>
      )}
    </>
  );
}
interface Selection {
  selected: string;
  ids: Set<string>;
  toggle: (id: string) => void;
  selectable: boolean;
  asset: string;
}
function Version({
  item,
  onSelect,
  selection,
}: {
  item: Item;
  onSelect: (id: string) => void;
  selection: Selection;
}) {
  const [open, setOpen] = useState(false);
  const expanded = open || selection.selected === item.componentId;
  const detail = useDetail(item.componentId!, expanded);
  return (
    <div style={{ paddingLeft: 24 }}>
      <div style={{ display: "flex", alignItems: "center" }}>
        {selection.selectable && (
          <input
            type="checkbox"
            aria-label={`Выбрать ${item.name} ${item.version}`}
            checked={selection.ids.has(item.componentId!)}
            onChange={() => selection.toggle(item.componentId!)}
          />
        )}
        <button
          style={row}
          aria-label={`Версия ${item.version}`}
          aria-expanded={expanded}
          onClick={() => {
            setOpen(!expanded);
            onSelect(expanded ? "" : item.componentId!);
          }}
        >
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          <Tag size={14} />
          {item.version}
        </button>
      </div>
      {expanded && (
        <div style={{ paddingLeft: 28 }}>
          {detail.isLoading && <p role="status">Загрузка файлов…</p>}
          {detail.isError && (
            <p role="alert">
              Не удалось загрузить файлы.{" "}
              <button onClick={() => void detail.refetch()}>Повторить</button>
            </p>
          )}
          {!detail.isError &&
            detail.data?.assets?.map((a) => (
              <button
                key={a.id || a.path}
                style={{
                  ...row,
                  fontWeight: a.path === selection.asset ? 700 : 400,
                }}
                onClick={() => onSelect(item.componentId!)}
              >
                <FileArchive size={14} />
                {a.path.split("/").pop()}
              </button>
            ))}
        </div>
      )}
    </div>
  );
}
function PackageRow({
  repository,
  name,
  onSelect,
  selection,
  linked,
}: {
  repository: string;
  name: string;
  onSelect: (id: string) => void;
  selection: Selection;
  linked?: Component;
}) {
  const [open, setOpen] = useState(!!linked);
  useEffect(() => {
    if (linked) setOpen(true);
  }, [linked?.id]);
  const branch = useBranch(repository, name, open);
  const versions = branch.data?.pages.flatMap((p) => p.items) ?? [];
  const pinned =
    linked && !versions.some((v) => v.componentId === linked.id)
      ? [{ name, version: linked.version, componentId: linked.id }]
      : [];
  return (
    <div>
      <button
        style={row}
        aria-label={`Пакет ${name}`}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <Package size={15} />
        {name}
      </button>
      {open && (
        <>
          {[...pinned, ...versions].map((v) => (
            <Version
              key={v.componentId}
              item={v}
              onSelect={onSelect}
              selection={selection}
            />
          ))}
          <BranchStatus branch={branch} />
        </>
      )}
    </div>
  );
}
function useSelection(
  highlightId: string,
  highlightAsset: string,
  selectable: boolean,
) {
  const [selected, setSelected] = useState(highlightId);
  const [ids, setIds] = useState<Set<string>>(new Set());
  useEffect(() => setSelected(highlightId), [highlightId]);
  const detail = useDetail(selected, !!selected);
  useEffect(() => {
    if (
      (detail.error as { response?: { status?: number } } | null)?.response
        ?.status === 404
    ) {
      setIds((current) => {
        const next = new Set(current);
        next.delete(selected);
        return next;
      });
      setSelected("");
    }
  }, [detail.error, selected]);
  const selection: Selection = {
    selected,
    ids,
    selectable,
    asset: highlightAsset,
    toggle: (id) =>
      setIds((current) => {
        const next = new Set(current);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      }),
  };
  return { selection, setSelected, clear: () => setIds(new Set()), detail };
}

function DetailPanel({
  detail,
  close,
  renderDetail,
}: {
  detail: ReturnType<typeof useDetail>;
  close: () => void;
  renderDetail: (component: Component, close: () => void) => ReactNode;
}) {
  if (detail.isLoading) return <p role="status">Загрузка сведений…</p>;
  if (detail.isError)
    return <p role="alert">Не удалось загрузить сведения о версии.</p>;
  return detail.data ? renderDetail(detail.data, close) : null;
}

export default function NpmBrowseTree({
  repository,
  renderDetail,
  highlightId = "",
  highlightAsset = "",
  selectable = false,
  onPromote,
}: {
  repository: string;
  renderDetail: (component: Component, close: () => void) => ReactNode;
  highlightId?: string;
  highlightAsset?: string;
  selectable?: boolean;
  onPromote: (ids: string[]) => void;
}) {
  const branch = useBranch(repository);
  const { selection, setSelected, clear, detail } = useSelection(
    highlightId,
    highlightAsset,
    selectable,
  );
  const { selected, ids } = selection;
  const items = branch.data?.pages.flatMap((p) => p.items) ?? [];
  const linked =
    !detail.isError && detail.data?.id === highlightId
      ? detail.data
      : undefined;
  const pinned =
    linked && !items.some((p) => p.name === linked.name)
      ? [{ name: linked.name }]
      : [];
  return (
    <div
      style={{
        display: "flex",
        gap: 16,
        minHeight: 0,
        flex: 1,
        overflow: "auto",
        alignItems: "flex-start",
        flexWrap: "wrap",
      }}
    >
      <div
        className="holo-card"
        style={{ flex: "2 1 420px", padding: 12, minWidth: 0 }}
      >
        {selectable && ids.size > 0 && (
          <div style={{ display: "flex", gap: 12, padding: 8 }}>
            <button onClick={() => onPromote([...ids])}>
              Promote selected ({ids.size})
            </button>
            <button onClick={clear}>Clear</button>
          </div>
        )}
        {[...pinned, ...items].map((p) => (
          <PackageRow
            key={p.name}
            repository={repository}
            name={p.name}
            onSelect={setSelected}
            selection={selection}
            linked={linked?.name === p.name ? linked : undefined}
          />
        ))}
        <BranchStatus branch={branch} />
        {!branch.isLoading &&
          !branch.isError &&
          items.length === 0 &&
          !branch.hasNextPage && <p>Пакетов пока нет.</p>}
      </div>
      {selected && (
        <DetailPanel
          detail={detail}
          close={() => setSelected("")}
          renderDetail={renderDetail}
        />
      )}
    </div>
  );
}
