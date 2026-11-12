import { expect, it } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import { QueryClient } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { renderWithProviders, seedAuthAsAdmin } from "@/test/renderUtils";
import { server } from "@/test/msw/server";
import BrowsePage from "./BrowsePage";

it("loads npm names, versions and files only when their parent is expanded", async () => {
  seedAuthAsAdmin();
  const calls: string[] = [];
  server.use(
    http.get("/service/rest/v1/repositories", () =>
      HttpResponse.json([
        { id: "npm", name: "npm-hosted", format: "npm", type: "hosted" },
      ]),
    ),
    http.get(
      "/api/v1/browse/repositories/npm-hosted/npm-tree",
      ({ request }) => {
        const name = new URL(request.url).searchParams.get("name") || "";
        calls.push(name || "packages");
        return HttpResponse.json({
          items: name
            ? [
                {
                  name,
                  version: "1.0.0",
                  componentId: "c1",
                  repository: "npm-hosted",
                },
              ]
            : [{ name: "@team/isenv" }],
          nextCursor: "",
        });
      },
    ),
    http.get("/service/rest/v1/components/c1", () => {
      calls.push("files");
      return HttpResponse.json({
        id: "c1",
        name: "@team/isenv",
        version: "1.0.0",
        format: "npm",
        repository: "npm-hosted",
        assets: [
          {
            id: "a1",
            path: "@team/isenv/-/isenv-1.0.0.tgz",
            fileSize: 23,
            contentType: "application/gzip",
          },
        ],
      });
    }),
  );
  renderWithProviders(<BrowsePage />, {
    routerProps: { initialEntries: ["/browse?repo=npm-hosted"] },
  });
  const user = userEvent.setup();
  await screen.findByRole("button", { name: "Пакет @team/isenv" });
  expect(calls).toEqual(["packages"]);
  await user.click(screen.getByRole("button", { name: "Пакет @team/isenv" }));
  await screen.findByRole("button", { name: "Версия 1.0.0" });
  expect(calls).toEqual(["packages", "@team/isenv"]);
  await user.click(screen.getByRole("button", { name: "Версия 1.0.0" }));
  await screen.findByRole("region", { name: "Component details" });
  expect(calls).toEqual(["packages", "@team/isenv", "files"]);
});

it("opens a version from Search, preserves multi-select and drops deleted details", async () => {
  seedAuthAsAdmin();
  let removed = false;
  server.use(
    http.get("/service/rest/v1/repositories", () =>
      HttpResponse.json([
        { id: "npm", name: "npm-hosted", format: "npm", type: "hosted" },
      ]),
    ),
    http.get("/api/v1/browse/repositories/npm-hosted/npm-tree", ({ request }) =>
      HttpResponse.json({
        items: new URL(request.url).searchParams.get("name")
          ? []
          : [{ name: "another-package" }],
        nextCursor: "",
      }),
    ),
    http.get("/service/rest/v1/components/c1", () =>
      removed
        ? new HttpResponse(null, { status: 404 })
        : HttpResponse.json({
            id: "c1",
            name: "@team/isenv",
            version: "1.0.0",
            format: "npm",
            repository: "npm-hosted",
            assets: [
              {
                id: "a1",
                path: "@team/isenv/-/isenv-1.0.0.tgz",
                fileSize: 23,
                contentType: "application/gzip",
              },
            ],
          }),
    ),
  );
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  renderWithProviders(<BrowsePage />, {
    queryClient,
    routerProps: {
      initialEntries: [
        "/browse?repo=npm-hosted&cid=c1&asset=%40team%2Fisenv%2F-%2Fisenv-1.0.0.tgz",
      ],
    },
  });
  await screen.findByRole("region", { name: "Component details" });
  const checkbox = await screen.findByRole("checkbox", {
    name: "Выбрать @team/isenv 1.0.0",
  });
  await userEvent.click(checkbox);
  expect(
    screen.getByRole("button", { name: "Promote selected (1)" }),
  ).toBeInTheDocument();
  removed = true;
  await act(async () => {
    await queryClient.invalidateQueries({ queryKey: ["npmComponentDetail"] });
  });
  await waitFor(() =>
    expect(
      screen.queryByRole("region", { name: "Component details" }),
    ).not.toBeInTheDocument(),
  );
  expect(
    screen.queryByRole("button", { name: "Promote selected (1)" }),
  ).not.toBeInTheDocument();
});

it("reports a failed branch, retries and fetches the next page only on request", async () => {
  seedAuthAsAdmin();
  let failed = true;
  const calls: string[] = [];
  server.use(
    http.get("/service/rest/v1/repositories", () =>
      HttpResponse.json([
        { id: "npm", name: "npm-hosted", format: "npm", type: "hosted" },
      ]),
    ),
    http.get(
      "/api/v1/browse/repositories/npm-hosted/npm-tree",
      ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor") || "";
        calls.push(cursor);
        if (failed) return new HttpResponse(null, { status: 500 });
        return HttpResponse.json({
          items: [{ name: cursor ? "z-package" : "@team/isenv" }],
          nextCursor: cursor ? "" : "next",
        });
      },
    ),
  );
  renderWithProviders(<BrowsePage />, {
    routerProps: { initialEntries: ["/browse?repo=npm-hosted"] },
  });
  await screen.findByRole("alert");
  expect(screen.queryByText("Пакетов пока нет.")).not.toBeInTheDocument();
  failed = false;
  await userEvent.click(screen.getByRole("button", { name: "Повторить" }));
  await screen.findByRole("button", { name: "Пакет @team/isenv" });
  expect(calls).toEqual(["", ""]);
  await userEvent.click(screen.getByRole("button", { name: "Загрузить ещё" }));
  await screen.findByRole("button", { name: "Пакет z-package" });
  expect(calls).toEqual(["", "", "next"]);
});
