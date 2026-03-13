import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi, type Mocked } from "vitest";
import App from "./App";
import { api } from "./lib/api";
import { makeSnapshot } from "./test/fixtures";

vi.mock("./lib/api", () => ({
  api: {
    eventsUrl: vi.fn(() => "/api/events"),
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

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly close = vi.fn();
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(readonly url: string) {
    MockEventSource.instances.push(this);
  }

  emit(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent<string>);
  }

  emitError() {
    this.onerror?.(new Event("error"));
  }

  static reset() {
    MockEventSource.instances = [];
  }
}

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
    MockEventSource.reset();
    vi.stubGlobal("EventSource", MockEventSource);
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

    expect((await screen.findAllByText("stocker-A")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("online-remote").length).toBeGreaterThan(0);
  });

  it("subscribes to live snapshot updates and refreshes runtime state", async () => {
    render(<App />);

    await screen.findByText("2 rules");
    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toBe("/api/events");

    const liveSnapshot = makeSnapshot();
    liveSnapshot.runtime.hsmsState = "NOT CONNECTED";
    liveSnapshot.state.mode = "online-local";
    liveSnapshot.messages.push({
      id: "msg-3",
      timestamp: "14:32:06.100",
      direction: "OUT",
      sf: "S6F11",
      label: "TRANSFER_COMPLETED",
      matchedRule: "accept transfer",
      matchedRuleId: "rule-1",
      detail: {
        stream: 6,
        function: 11,
        wbit: true,
        body: "<A \"TRANSFER_COMPLETED\">",
        rawSml: "S6F11 W <A \"TRANSFER_COMPLETED\">",
      },
      evaluations: [],
    });

    await act(async () => {
      MockEventSource.instances[0].emit(liveSnapshot);
    });

    expect(await screen.findByText("Messages: 3")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "2", ctrlKey: true });
    expect((await screen.findAllByText("online-local")).length).toBeGreaterThan(0);
  });

  it("surfaces runtime transport warnings from live snapshots", async () => {
    render(<App />);

    await screen.findByText("2 rules");

    const liveSnapshot = makeSnapshot();
    liveSnapshot.runtime.hsmsState = "CONNECTING";
    liveSnapshot.runtime.lastError = "connection refused";

    await act(async () => {
      MockEventSource.instances[0].emit(liveSnapshot);
    });

    expect(await screen.findByText("Transport issue: connection refused")).toBeInTheDocument();
    expect(screen.getAllByText("Transport issue").length).toBeGreaterThan(0);
  });

  it("shows a reconnecting warning when the live update stream drops", async () => {
    render(<App />);

    await screen.findByText("2 rules");

    await act(async () => {
      MockEventSource.instances[0].emitError();
    });

    expect(await screen.findByText("Live updates disconnected. Reconnecting…")).toBeInTheDocument();
  });

  it("shows restart required only for unapplied HSMS connection changes", async () => {
    const user = userEvent.setup();
    const snapshot = makeSnapshot();
    snapshot.runtime.dirty = true;
    snapshot.runtime.restartRequired = false;
    configureApi(snapshot);

    render(<App />);

    await screen.findByText("2 rules");
    await user.click(screen.getByRole("button", { name: /HSMS/i }));

    expect(screen.queryByText("restart required")).not.toBeInTheDocument();

    const liveSnapshot = makeSnapshot();
    liveSnapshot.runtime.dirty = false;
    liveSnapshot.runtime.restartRequired = true;

    await act(async () => {
      MockEventSource.instances[0].emit(liveSnapshot);
    });

    expect(await screen.findByText("restart required")).toBeInTheDocument();
  });

  it("lets the user hide and re-show the message log panel", async () => {
    const user = userEvent.setup();
    render(<App />);

    await screen.findByText("2 rules");
    expect(screen.getByText("Message Log")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Hide log" }));

    expect(screen.queryByText("Message Log")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show log" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show log" }));

    expect(await screen.findByText("Message Log")).toBeInTheDocument();
  });

  it("shows the overview focus banner", async () => {
    render(<App />);

    await screen.findByText("2 rules");

    expect(screen.getByText("Commit or discard the current config edits")).toBeInTheDocument();
  });
});
