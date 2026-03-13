import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MessageMonitor } from "./MessageMonitor";
import { makeSnapshot } from "../test/fixtures";

function defineScrollMetrics(element: HTMLDivElement, scrollTop: number) {
  Object.defineProperty(element, "scrollHeight", {
    configurable: true,
    value: 1200,
  });
  Object.defineProperty(element, "clientHeight", {
    configurable: true,
    value: 300,
  });
  Object.defineProperty(element, "scrollTop", {
    configurable: true,
    writable: true,
    value: scrollTop,
  });
}

describe("MessageMonitor", () => {
  it("pauses auto-tail when scrolled up and lets the user jump to latest messages", async () => {
    const snapshot = makeSnapshot();
    const onSelectMessage = vi.fn();
    const onChangeDetailTab = vi.fn();
    const onJumpToRule = vi.fn();
    const onClearLog = vi.fn();
    const onHide = vi.fn();

    const { container, rerender } = render(
      <MessageMonitor
        messages={snapshot.messages}
        selectedMessageId={null}
        detailTab="decoded"
        onSelectMessage={onSelectMessage}
        onChangeDetailTab={onChangeDetailTab}
        onJumpToRule={onJumpToRule}
        onClearLog={onClearLog}
        onHide={onHide}
      />,
    );

    const scrollBox = container.querySelector(".message-scroll");
    if (!(scrollBox instanceof HTMLDivElement)) {
      throw new Error("message scroll container not found");
    }

    const scrollTo = vi.fn(({ top }: { top: number }) => {
      scrollBox.scrollTop = top;
    });
    Object.defineProperty(scrollBox, "scrollTo", {
      configurable: true,
      value: scrollTo,
    });
    defineScrollMetrics(scrollBox, 900);

    scrollBox.scrollTop = 0;
    fireEvent.scroll(scrollBox);

    expect(screen.getByText("Paused")).toBeInTheDocument();

    const nextMessages = [
      ...snapshot.messages,
      {
        id: "msg-3",
        timestamp: "14:32:06.100",
        direction: "OUT" as const,
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
      },
    ];

    rerender(
      <MessageMonitor
        messages={nextMessages}
        selectedMessageId={null}
        detailTab="decoded"
        onSelectMessage={onSelectMessage}
        onChangeDetailTab={onChangeDetailTab}
        onJumpToRule={onJumpToRule}
        onClearLog={onClearLog}
        onHide={onHide}
      />,
    );

    const jumpButton = await screen.findByRole("button", { name: "Down 1 new message" });
    fireEvent.click(jumpButton);

    await waitFor(() => {
      expect(scrollTo).toHaveBeenCalled();
    });
    expect(screen.queryByRole("button", { name: "Down 1 new message" })).not.toBeInTheDocument();
    expect(screen.getByText("Live tail")).toBeInTheDocument();
  });

  it("filters visible traffic by source, direction, and search text", () => {
    const snapshot = makeSnapshot();
    snapshot.messages.unshift({
      id: "msg-0",
      timestamp: "14:32:00.000",
      direction: "IN",
      sf: "S1F13",
      label: "Establish Comm",
      detail: {
        stream: 1,
        function: 13,
        wbit: true,
        body: "L:0",
        rawSml: "S1F13 W L:0",
      },
      evaluations: [],
    });

    render(
      <MessageMonitor
        messages={snapshot.messages}
        selectedMessageId={null}
        detailTab="decoded"
        onSelectMessage={vi.fn()}
        onChangeDetailTab={vi.fn()}
        onJumpToRule={vi.fn()}
        onClearLog={vi.fn()}
        onHide={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "System only" }));
    expect(screen.getByText("Establish Comm")).toBeInTheDocument();
    expect(screen.queryByText("Remote Command: TRANSFER")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "All sources" }));
    fireEvent.click(screen.getByRole("button", { name: "Outgoing" }));
    fireEvent.change(screen.getByLabelText("Search messages"), { target: { value: "Ack" } });

    expect(screen.getByText("Remote Cmd Ack")).toBeInTheDocument();
    expect(screen.queryByText("Establish Comm")).not.toBeInTheDocument();
    expect(screen.queryByText("Remote Command: TRANSFER")).not.toBeInTheDocument();
    expect(screen.getByText("1/3 shown")).toBeInTheDocument();
  });
});
