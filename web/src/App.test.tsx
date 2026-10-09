import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { demoData } from "./api";

describe("Suppervisor PWA", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    localStorage.clear();
    sessionStorage.clear();
  });

  it("keeps the complete dinner workflow usable with labelled demo data", async () => {
    render(<App />);
    expect(
      await screen.findByRole("heading", {
        name: "What should dinner do for you?",
      }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByText(/Previewing seeded demo data/),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Recipe" })[0]);
    expect(
      screen.getByRole("heading", { name: "One-pan tomato lentil orzo" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: /Five calm minutes/ }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Open shopping/ }));
    expect(
      screen.getByText(/0 of 5 tucked into the basket/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: /orzo/ }));
    expect(
      screen.getByText(/1 of 5 tucked into the basket/),
    ).toBeInTheDocument();
  });

  it("shows distinct profiles, mobile history, and management", async () => {
    render(<App />);
    await screen.findByRole("heading", {
      name: "What should dinner do for you?",
    });
    expect(screen.getAllByRole("button", { name: "History" })).toHaveLength(2);
    fireEvent.click(screen.getAllByRole("button", { name: "Profile" })[0]);
    expect(screen.getByRole("heading", { name: "Alan" })).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Partner" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Manage" }));
    expect(
      screen.getByRole("tab", { name: "preferences" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "corrections" }),
    ).toBeInTheDocument();
  });

  it("uses accessible adjustment and structured individual ratings", async () => {
    render(<App />);
    await screen.findByRole("heading", {
      name: "What should dinner do for you?",
    });
    fireEvent.click(screen.getByRole("button", { name: "Adjust" }));
    expect(
      screen.getByRole("dialog", { name: "What should change?" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getAllByRole("button", { name: "Cooking" })[0]);
    fireEvent.click(screen.getByRole("button", { name: /Next step/ }));
    fireEvent.click(screen.getByRole("button", { name: /Next step/ }));
    fireEvent.click(screen.getByRole("button", { name: /Dinner done/ }));
    expect(screen.getByRole("group", { name: "Taste" })).toBeInTheDocument();
    expect(
      screen.getByRole("group", { name: "Reheating" }),
    ).toBeInTheDocument();
  });

  it("shows a production OIDC sign-in gate", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            authRequired: true,
            oidcIssuer: "https://id.example.test/realms/home",
            oidcClientId: "suppervisor",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    render(<App />);
    expect(
      await screen.findByRole("heading", {
        name: "Come in. Dinner’s waiting.",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("shows every candidate from the latest ranked planning batch", async () => {
    const candidates = ["Bean supper", "Lentil supper", "Chickpea supper"].map(
      (title, index) => ({
        ...demoData.decisions[0],
        id: `new-${index}`,
        candidate: {
          ...demoData.decisions[0].candidate,
          id: `recipe-${index}`,
          title,
        },
        status: "proposed" as const,
      }),
    );
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/public/config")) {
          return Promise.resolve(
            new Response(JSON.stringify({ authRequired: false }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        const body = url.endsWith("/plans")
          ? { decisions: candidates }
          : demoData;
        return Promise.resolve(
          new Response(JSON.stringify(body), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }),
    );
    render(<App />);
    await screen.findByRole("heading", {
      name: "What should dinner do for you?",
    });
    fireEvent.click(screen.getByRole("button", { name: "Find dinner" }));
    for (const title of ["Bean supper", "Lentil supper", "Chickpea supper"]) {
      expect(
        await screen.findByRole("heading", { name: title }),
      ).toBeInTheDocument();
    }
  });
});
