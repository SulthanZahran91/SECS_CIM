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
}

export function MessageMonitor({
  messages,
  selectedMessageId,
  detailTab,
  onSelectMessage,
  onChangeDetailTab,
  onJumpToRule,
  onClearLog,
}: MessageMonitorProps) {
  const selectedMessage = selectedMessageId ? messages.find((message) => message.id === selectedMessageId) ?? null : null;

  return (
    <div className="monitor-shell">
      <div className="monitor-list">
        <SectionHeader right={<button className="text-button" onClick={onClearLog} type="button">Clear log</button>}>
          Message Log
        </SectionHeader>
        <div className="message-header">
          <span className="time-col">Time</span>
          <span className="dir-col">Dir</span>
          <span className="sf-col">SxFy</span>
          <span className="info-col">Info</span>
          <span className="rule-col">Matched Rule</span>
        </div>
        <div className="message-scroll">
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
                    <span>
                      <span className="subtle-text">Stream:</span> <span className="sf-text">{selectedMessage.detail.stream}</span>
                    </span>
                    <span>
                      <span className="subtle-text">Function:</span>{" "}
                      <span className="sf-text">{selectedMessage.detail.function}</span>
                    </span>
                    <span>
                      <span className="subtle-text">W-bit:</span>{" "}
                      <span className={selectedMessage.detail.wbit ? "text-green" : "subtle-text"}>
                        {selectedMessage.detail.wbit ? "yes" : "no"}
                      </span>
                    </span>
                    <span>
                      <span className="subtle-text">Direction:</span>{" "}
                      <Badge tone={selectedMessage.direction === "IN" ? "green" : "blue"}>
                        {selectedMessage.direction}
                      </Badge>
                    </span>
                  </div>
                  <div className="detail-label">Body (SML tree)</div>
                  <pre className="detail-code">{selectedMessage.detail.body}</pre>
                </div>
              ) : null}

              {detailTab === "raw" ? <pre className="detail-code">{selectedMessage.detail.rawSml}</pre> : null}

              {detailTab === "rule" ? (
                selectedMessage.matchedRule ? (
                  <div className="matched-rule-panel">
                    <button
                      className="text-link"
                      onClick={() => onJumpToRule(selectedMessage.matchedRuleId)}
                      type="button"
                    >
                      Rule: {selectedMessage.matchedRule}
                    </button>
                    {selectedMessage.evaluations?.length ? (
                      <div className="stack-list">
                        {selectedMessage.evaluations.map((evaluation, index) => (
                          <div className="evaluation-row" key={`${selectedMessage.id}-evaluation-${index}`}>
                            <span className="mono">{evaluation.field}</span>
                            <span className="subtle-text">expected</span>
                            <span>{evaluation.expected}</span>
                            <span className="subtle-text">actual</span>
                            <span>{evaluation.actual}</span>
                            <Badge tone={evaluation.passed ? "green" : "red"}>
                              {evaluation.passed ? "pass" : "fail"}
                            </Badge>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="empty-copy">This message was generated by a matched rule.</div>
                    )}
                  </div>
                ) : (
                  <div className="empty-copy">No rule matched. This was a session or handshake message.</div>
                )
              ) : null}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

