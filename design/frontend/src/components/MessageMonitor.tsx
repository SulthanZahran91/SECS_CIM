import { useEffect, useRef, useState } from "react";
import { Badge, SectionHeader, TabButton } from "./ui";
import type { DetailTab, MessageRecord } from "../types";

interface MessageMonitorProps {
  messages: MessageRecord[];
  selectedMessageId: string | null;
  detailTab: DetailTab;
  onSelectMessage: (id: string | null) => void;
  onChangeDetailTab: (tab: DetailTab) => void;
  onJumpToRule: (ruleId: string | undefined) => void;
  onClearLog: () => void;
  onHide: () => void;
}

export function MessageMonitor({
  messages,
  selectedMessageId,
  detailTab,
  onSelectMessage,
  onChangeDetailTab,
  onJumpToRule,
  onClearLog,
  onHide,
}: MessageMonitorProps) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const previousCountRef = useRef(messages.length);
  const [isPinnedToBottom, setIsPinnedToBottom] = useState(true);
  const [pendingMessageCount, setPendingMessageCount] = useState(0);
  const selectedMessage = selectedMessageId ? messages.find((message) => message.id === selectedMessageId) ?? null : null;

  useEffect(() => {
    const nextCount = messages.length;
    const previousCount = previousCountRef.current;
    previousCountRef.current = nextCount;

    if (nextCount === 0) {
      setPendingMessageCount(0);
      return;
    }

    if (nextCount <= previousCount) {
      if (isPinnedToBottom) {
        scrollToBottom(scrollRef.current);
      }
      setPendingMessageCount(0);
      return;
    }

    if (isPinnedToBottom) {
      scrollToBottom(scrollRef.current);
      setPendingMessageCount(0);
      return;
    }

    setPendingMessageCount((current) => current + (nextCount - previousCount));
  }, [isPinnedToBottom, messages]);

  function handleScroll() {
    const atBottom = isNearBottom(scrollRef.current);
    setIsPinnedToBottom(atBottom);
    if (atBottom) {
      setPendingMessageCount(0);
    }
  }

  function jumpToLatest() {
    scrollToBottom(scrollRef.current);
    setIsPinnedToBottom(true);
    setPendingMessageCount(0);
  }

  return (
    <div className="monitor-shell">
      <div className="monitor-list">
        <SectionHeader
          right={
            <div className="monitor-actions">
              <Badge tone={isPinnedToBottom ? "green" : "yellow"}>{isPinnedToBottom ? "Live tail" : "Paused"}</Badge>
              <button className="text-button" onClick={onHide} type="button">Hide log</button>
              <button className="text-button" onClick={onClearLog} type="button">Clear log</button>
            </div>
          }
        >
          Message Log
        </SectionHeader>
        <div className="message-header">
          <span className="time-col">Time</span>
          <span className="dir-col">Dir</span>
          <span className="sf-col">SxFy</span>
          <span className="info-col">Info</span>
          <span className="rule-col">Matched Rule</span>
        </div>
        <div className="message-scroll-wrap">
          <div className="message-scroll" onScroll={handleScroll} ref={scrollRef}>
            {messages.length === 0 ? <div className="empty-copy padded">Message log is empty.</div> : null}
            {messages.map((message) => {
              const selected = message.id === selectedMessageId;
              return (
                <button
                  className={`message-row ${selected ? "selected" : ""}`}
                  key={message.id}
                  onClick={() => onSelectMessage(selected ? null : message.id)}
                  type="button"
                >
                  <span className="time-col subtle-text">{message.timestamp}</span>
                  <span className="dir-col">
                    <Badge tone={message.direction === "IN" ? "green" : "blue"}>{message.direction}</Badge>
                  </span>
                  <span className="sf-col sf-text">{message.sf}</span>
                  <span className="info-col">{message.label}</span>
                  <span className="rule-col matched-rule-text">{message.matchedRule ?? "—"}</span>
                </button>
              );
            })}
          </div>
          {!isPinnedToBottom && pendingMessageCount > 0 ? (
            <div className="tail-indicator">
              <button className="tail-button" onClick={jumpToLatest} type="button">
                ↓ {pendingMessageCount} new {pendingMessageCount === 1 ? "message" : "messages"}
              </button>
            </div>
          ) : null}
        </div>
      </div>

      <div className={`detail-pane ${selectedMessage ? "open" : ""}`}>
        {selectedMessage ? (
          <>
            <div className="tab-row detail-tabs">
              <TabButton active={detailTab === "decoded"} onClick={() => onChangeDetailTab("decoded")}>
                Decoded
              </TabButton>
              <TabButton active={detailTab === "raw"} onClick={() => onChangeDetailTab("raw")}>
                Raw SML
              </TabButton>
              <TabButton active={detailTab === "rule"} onClick={() => onChangeDetailTab("rule")}>
                Matched Rule
              </TabButton>
            </div>
            <div className="detail-body">
              {detailTab === "decoded" ? (
                <div className="detail-grid">
                  <div className="detail-summary">
                    <div className="field-group">
                      <span className="detail-label">Stream</span>
                      <span className="sf-text" style={{ fontSize: 16 }}>{selectedMessage.detail.stream}</span>
                    </div>
                    <div className="field-group">
                      <span className="detail-label">Function</span>
                      <span className="sf-text" style={{ fontSize: 16 }}>{selectedMessage.detail.function}</span>
                    </div>
                    <div className="field-group">
                      <span className="detail-label">W-bit</span>
                      <span className={selectedMessage.detail.wbit ? "text-green" : "subtle-text"}>
                        {selectedMessage.detail.wbit ? "SET (Wait)" : "NOT SET"}
                      </span>
                    </div>
                    <div className="field-group">
                      <span className="detail-label">Direction</span>
                      <Badge tone={selectedMessage.direction === "IN" ? "green" : "blue"}>
                        {selectedMessage.direction === "IN" ? "INCOMING" : "OUTGOING"}
                      </Badge>
                    </div>
                  </div>
                  <div className="detail-label" style={{ marginTop: 8 }}>Body (SML Tree)</div>
                  <pre className="detail-code">{selectedMessage.detail.body}</pre>
                </div>
              ) : null}

              {detailTab === "raw" ? (
                <div className="detail-grid">
                  <div className="detail-label">Raw SML Representation</div>
                  <pre className="detail-code">{selectedMessage.detail.rawSml}</pre>
                </div>
              ) : null}

              {detailTab === "rule" ? (
                <div className="matched-rule-panel">
                  {selectedMessage.matchedRule ? (
                    <>
                      <div className="detail-label">Triggered Rule</div>
                      <button
                        className="text-link"
                        onClick={() => onJumpToRule(selectedMessage.matchedRuleId)}
                        type="button"
                        style={{ fontSize: 14, textAlign: "left" }}
                      >
                        {selectedMessage.matchedRule}
                      </button>
                      
                      {selectedMessage.evaluations?.length ? (
                        <>
                          <div className="detail-label" style={{ marginTop: 12 }}>Condition Evaluations</div>
                          <div className="stack-list">
                            {selectedMessage.evaluations.map((evaluation, index) => (
                              <div className="condition-row" key={`${selectedMessage.id}-evaluation-${index}`} style={{ background: "rgba(0,0,0,0.2)" }}>
                                <div className="field-group" style={{ flex: 1 }}>
                                  <span className="field-label">Field</span>
                                  <span className="mono">{evaluation.field}</span>
                                </div>
                                <div className="field-group" style={{ flex: 1 }}>
                                  <span className="field-label">Expected</span>
                                  <span>{evaluation.expected}</span>
                                </div>
                                <div className="field-group" style={{ flex: 1 }}>
                                  <span className="field-label">Actual</span>
                                  <span className={evaluation.passed ? "text-green" : "text-red"}>{evaluation.actual}</span>
                                </div>
                                <Badge tone={evaluation.passed ? "green" : "red"}>
                                  {evaluation.passed ? "PASS" : "FAIL"}
                                </Badge>
                              </div>
                            ))}
                          </div>
                        </>
                      ) : (
                        <div className="empty-copy">This message was a direct response or side effect (no conditions evaluated).</div>
                      )}
                    </>
                  ) : (
                    <div className="empty-copy">No rule match recorded for this message. It may be a system-level handshake (S1F13, S1F1, etc.).</div>
                  )}
                </div>
              ) : null}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

function isNearBottom(element: HTMLDivElement | null): boolean {
  if (!element) {
    return true;
  }

  const remaining = element.scrollHeight - element.clientHeight - element.scrollTop;
  return remaining <= 50;
}

function scrollToBottom(element: HTMLDivElement | null) {
  if (!element) {
    return;
  }

  const top = element.scrollHeight;
  if (typeof element.scrollTo === "function") {
    element.scrollTo({ top, behavior: "auto" });
    return;
  }

  element.scrollTop = top;
}
