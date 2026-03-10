import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi, type Mocked } from "vitest";
import App from "./App";
import { api } from "./lib/api";
import { makeSnapshot } from "./test/fixtures";

vi.mock("./lib/api", () => ({
  api: {
    bootstrap: vi.fn(),
    toggleRuntime: vi.fn(),
    saveConfig: vi.fn(),
    reloadConfig: vi.fn(),
    clearLog: vi.fn(),
    updateHsms: vi.fn(),
    updateDevice: vi.fn(),
    createRule: vi.fn(),
    updateRule: vi.fn(),
    duplicateRule: vi.fn(),
    deleteRule: vi.fn(),
    moveRule: vi.fn(),
  },
}));

const mockedApi = api as Mocked<typeof api>;

function configureApi(snapshot = makeSnapshot()) {
  mockedApi.bootstrap.mockResolvedValue(snapshot);
  mockedApi.toggleRuntime.mockResolvedValue({
    ...snapshot,
    runtime: { ...snapshot.runtime, listening: false, hsmsState: "NOT CONNECTED" },
  });
  mockedApi.saveConfig.mockResolvedValue({
    ...snapshot,
    runtime: { ...snapshot.runtime, dirty: false },
  });
  mockedApi.reloadConfig.mockResolvedValue({
    ...snapshot,
    runtime: { ...snapshot.runtime, dirty: false },
  });
  mockedApi.clearLog.mockResolvedValue({
    ...snapshot,
    messages: [],
  });
  mockedApi.updateHsms.mockResolvedValue(snapshot);
  mockedApi.updateDevice.mockResolvedValue(snapshot);
  mockedApi.createRule.mockResolvedValue({
    ...snapshot,
    rules: [
      ...snapshot.rules,
      {
        id: "rule-3",
        name: "new rule",
        enabled: true,
        match: { stream: 0, function: 0, rcmd: "" },
        conditions: [],
        reply: { stream: 0, function: 0, ack: 0 },
        actions: [],
      },
    ],
  });
  mockedApi.updateRule.mockResolvedValue(snapshot);
  mockedApi.duplicateRule.mockResolvedValue(snapshot);
  mockedApi.deleteRule.mockResolvedValue(snapshot);
  mockedApi.moveRule.mockResolvedValue(snapshot);
}

describe("App", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    configureApi();
  });

  it("loads the bootstrap snapshot and renders the simulator shell", async () => {
    render(<App />);

    expect(screen.getByText("Loading SECSIM scaffold…")).toBeInTheDocument();
    expect(await screen.findByText("2 rules")).toBeInTheDocument();
    expect(screen.getByText("Messages: 2")).toBeInTheDocument();
    expect(mockedApi.bootstrap).toHaveBeenCalledTimes(1);
  });

  it("shows bootstrap errors when initial load fails", async () => {
    mockedApi.bootstrap.mockRejectedValueOnce(new Error("bootstrap failed"));

    render(<App />);

    expect(await screen.findByRole("alert")).toHaveTextContent("bootstrap failed");
  });

  it("normalizes malformed snapshot collections before rendering", async () => {
    mockedApi.bootstrap.mockResolvedValueOnce({
      ...makeSnapshot(),
      rules: null as never,
      messages: null as never,
    });

    render(<App />);

    expect(await screen.findByText("0 rules")).toBeInTheDocument();
    expect(screen.getByText("Messages: 0")).toBeInTheDocument();
  });

  it("saves config from the toolbar and shows a notice", async () => {
    const user = userEvent.setup();
    render(<App />);

    await screen.findByText("2 rules");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(mockedApi.saveConfig).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByText("Config saved")).toBeInTheDocument();
  });

  it("switches tabs with keyboard shortcuts", async () => {
    render(<App />);

    await screen.findByText("2 rules");
    fireEvent.keyDown(window, { key: "2", ctrlKey: true });

    expect(await screen.findByText("stocker-A")).toBeInTheDocument();
    expect(screen.getByText("online-remote")).toBeInTheDocument();
  });
});
