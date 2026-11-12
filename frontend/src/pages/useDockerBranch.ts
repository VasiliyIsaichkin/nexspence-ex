import { useInfiniteQuery } from "@tanstack/react-query";
import { nexspenceApi } from "@/api/client";
import { useAuthStore } from "@/store/authStore";

export interface DockerTreeNode {
  kind: "folder" | "tag" | "manifest" | "blob";
  label: string;
  path: string;
  imageRef?: string;
  version?: string;
  componentId?: string;
  artifactType?: string;
  children?: DockerTreeNode[];
  lazy?: boolean;
}
interface BranchPage {
  root: DockerTreeNode;
  nextCursor?: string;
}

export function useBrowseSession() {
  return useAuthStore(
    (s) =>
      `${s.user?.id ?? "anonymous"}:${s.user?.roles.join(",") ?? ""}:${s.token !== null}`,
  );
}

export function useDockerBranch(
  repository: string,
  path: string,
  enabled: boolean,
) {
  const session = useBrowseSession();
  const query = useInfiniteQuery({
    queryKey: ["dockerBrowseTree", repository, session, path],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      nexspenceApi
        .getDockerBrowseTree(repository, {
          path,
          limit: 100,
          ...(pageParam ? { cursor: pageParam } : {}),
        })
        .then((r) => r.data as BranchPage),
    getNextPageParam: (page: BranchPage) => page.nextCursor || undefined,
    enabled,
    retry: false,
  });
  const pages = query.data?.pages;
  const root = pages?.[0]?.root;
  return {
    ...query,
    root: root
      ? { ...root, children: pages!.flatMap((p) => p.root.children ?? []) }
      : undefined,
  };
}
