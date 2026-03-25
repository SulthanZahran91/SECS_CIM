import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { HsmsTab } from "./HsmsTab";
import { makeSnapshot } from "../test/fixtures";

describe("HsmsTab", () => {
  it("shows validation issues and restart required indicator", async () => {
    const user = userEvent.setup();
    const snapshot = makeSnapshot();
    snapshot.hsms.ip = "";
    snapshot.hsms.port = 0;
    snapshot.hsms.handshake.autoHostStartup = true;
    snapshot.hsms.handshake.hostStartupProfile = "conveyor";
    snapshot.device.name = "";

    render(
      <HsmsTab
        hsms={snapshot.hsms}
        device={snapshot.device}
        restartRequired
        onChangeHsms={vi.fn()}
        onChangeDevice={vi.fn()}
      />,
    );

    // Validation issues section exists - expand it
    const issuesHeader = screen.getByText("Validation Issues");
    await user.click(issuesHeader);
    expect(screen.getAllByText("Address is required.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Use a TCP port between 1 and 65535.").length).toBeGreaterThan(0);
    expect(screen.getByText("restart required")).toBeInTheDocument();
  });

  it("shows clearly differentiated startup profile options", async () => {
    const user = userEvent.setup();
    const snapshot = makeSnapshot();

    render(
      <HsmsTab
        hsms={snapshot.hsms}
        device={snapshot.device}
        restartRequired={false}
        onChangeHsms={vi.fn()}
        onChangeDevice={vi.fn()}
      />,
    );

    await user.click(screen.getByText("Handshake"));
    expect(screen.getByText("Host startup profile")).toBeInTheDocument();
    expect(screen.getByText("Stocker / Minimal")).toBeInTheDocument();
    expect(screen.getByText("Conveyor / Captured Trace")).toBeInTheDocument();
    expect(screen.getByText("S1F17 Request On-Line")).toBeInTheDocument();
    expect(screen.getByText("No automated host bring-up runs after HSMS select.")).toBeInTheDocument();
  });

  it("describes session id as the on-wire HSMS header value", () => {
    const snapshot = makeSnapshot();

    render(
      <HsmsTab
        hsms={snapshot.hsms}
        device={snapshot.device}
        restartRequired={false}
        onChangeHsms={vi.fn()}
        onChangeDevice={vi.fn()}
      />,
    );

    expect(screen.getByText("Session ID")).toBeInTheDocument();
    expect(screen.getByText("Device ID")).toBeInTheDocument();
    expect(
      screen.getByText("The HSMS wire header uses Session ID in bytes 4-5. Device ID is preserved in config, but it is not written into the HSMS header."),
    ).toBeInTheDocument();
  });

  it("flags wildcard addresses as invalid in active mode", async () => {
    const user = userEvent.setup();
    const snapshot = makeSnapshot();
    snapshot.hsms.mode = "active";
    snapshot.hsms.ip = "0.0.0.0";

    render(
      <HsmsTab
        hsms={snapshot.hsms}
        device={snapshot.device}
        restartRequired={false}
        onChangeHsms={vi.fn()}
        onChangeDevice={vi.fn()}
      />,
    );

    await user.click(screen.getByText("Validation Issues"));
    expect(
      screen.getAllByText("Active mode must target a concrete host address. Use 127.0.0.1 for a local passive equipment."),
    ).toHaveLength(2);
  });
});
