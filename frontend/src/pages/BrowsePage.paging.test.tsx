import { expect, it } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { renderWithProviders, seedAuthAsAdmin } from "@/test/renderUtils";
import { server } from "@/test/msw/server";
import BrowsePage from "./BrowsePage";

it("loads a folder only when expanded, then exposes the next page", async () => {
  seedAuthAsAdmin();
  const calls: string[] = [];
  server.use(
    http.get("/service/rest/v1/repositories", () =>
      HttpResponse.json([
        { id: "r", name: "docker-hosted", format: "docker", type: "hosted" },
      ]),
    ),
  );
  server.use(
    http.get(
      "/api/v1/browse/repositories/docker-hosted/docker-tree",
      ({ request }) => {
        const u = new URL(request.url),
          path = u.searchParams.get("path") ?? "/";
        calls.push(path);
        const label =
          path === "/"
            ? "team-b"
            : u.searchParams.get("cursor")
              ? "second"
              : "first";
        return HttpResponse.json({
          root: {
            kind: "folder",
            path,
            children: [
              {
                kind: "folder",
                label,
                path: `${path === "/" ? "" : path}/${label}`,
                lazy: true,
              },
            ],
          },
          nextCursor:
            path === "/team-b" && !u.searchParams.get("cursor") ? "next" : "",
        });
      },
    ),
  );
  renderWithProviders(<BrowsePage />, {
    routerProps: { initialEntries: ["/browse?repo=docker-hosted"] },
  });
  const user = userEvent.setup();
  await screen.findByText("team-b");
  expect(calls).toEqual(["/"]);
  await user.click(screen.getByText("team-b"));
  await screen.findByText("first");
  await user.click(screen.getByRole("button", { name: "Загрузить ещё" }));
  await screen.findByText("second");
  expect(screen.getByText("first")).toBeInTheDocument();
});

it("shows a failed tree request as an error, not an empty repository", async () => {
  seedAuthAsAdmin();
  server.use(
    http.get("/service/rest/v1/repositories", () =>
      HttpResponse.json([
        { id: "r", name: "docker-hosted", format: "docker", type: "hosted" },
      ]),
    ),
  );
  server.use(
    http.get("/api/v1/browse/repositories/docker-hosted/docker-tree", () =>
      HttpResponse.json({ error: "unavailable" }, { status: 500 }),
    ),
  );
  renderWithProviders(<BrowsePage />, {
    routerProps: { initialEntries: ["/browse?repo=docker-hosted"] },
  });
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Не удалось загрузить дерево",
  );
  expect(screen.queryByText(/No Docker metadata/)).not.toBeInTheDocument();
});
