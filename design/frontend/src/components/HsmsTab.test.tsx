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
});
