import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HsmsTab } from "./HsmsTab";
import { makeSnapshot } from "../test/fixtures";

describe("HsmsTab", () => {
  it("shows validation issues and explains save versus restart impact", () => {
    const snapshot = makeSnapshot();
    snapshot.hsms.ip = "";
    snapshot.hsms.port = 0;
    snapshot.hsms.handshake.autoHostStartup = true;
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

    expect(screen.getByText("Connection Readiness")).toBeInTheDocument();
    expect(screen.getByText(/Save writes the working config to disk/)).toBeInTheDocument();
    expect(screen.getAllByText("Address is required.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Use a TCP port between 1 and 65535.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Device name is required.").length).toBeGreaterThan(0);
    expect(screen.getByText("Active-mode host startup only runs when the connection mode is active.")).toBeInTheDocument();
    expect(screen.getByText("restart required")).toBeInTheDocument();
  });
});
