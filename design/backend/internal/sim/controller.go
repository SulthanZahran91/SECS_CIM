package sim

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
	"secsim/design/backend/internal/store"
)

var ErrNotRunning = errors.New("simulator is not running")

type hostBootstrapPhase int

const (
	hostBootstrapIdle hostBootstrapPhase = iota
	hostBootstrapAwaitingS1F14
	hostBootstrapAwaitingS1F18
	hostBootstrapAwaitingS2F32
	hostBootstrapReady
)

type Controller struct {
	store         *store.Store
	mu            sync.Mutex
	cancel        context.CancelFunc
	transport     *hsms.Manager
	hostBootstrap hostBootstrapPhase
}

type Status struct {
	Running      bool   `json:"running"`
	HSMSState    string `json:"hsmsState"`
	MessageCount int    `json:"messageCount"`
	ConfigFile   string `json:"configFile"`
	Dirty        bool   `json:"dirty"`
	LastError    string `json:"lastError,omitempty"`
}

func New(state *store.Store) *Controller {
	return &Controller{store: state}
}

func (c *Controller) Start() (model.Snapshot, error) {
	c.mu.Lock()
	if c.cancel != nil {
		snapshot := c.store.Snapshot()
		c.mu.Unlock()
		return snapshot, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	appliedConfig := c.store.ConfigSnapshot().HSMS
	transport := hsms.NewManager(appliedConfig, hsms.Handlers{
		OnData:        c.handleDataMessage,
		OnStateChange: c.updateRuntimeState,
		OnError: func(err error) {
			log.Printf("HSMS runtime error: %v", err)
			c.store.SetRuntimeError(normalizeRuntimeError(err))
		},
	})
	c.cancel = cancel
	c.transport = transport
	c.hostBootstrap = hostBootstrapIdle
	c.mu.Unlock()

	c.store.RecordAppliedHSMS(appliedConfig)
	c.store.SetRuntime(true, "NOT CONNECTED")

	if err := transport.Start(ctx); err != nil {
		cancel()
		c.mu.Lock()
		c.cancel = nil
		c.transport = nil
		c.mu.Unlock()
		return c.store.SetRuntime(false, "NOT CONNECTED"), err
	}
	go c.schedulerLoop(ctx)

	return c.store.Snapshot(), nil
}

func (c *Controller) Stop() model.Snapshot {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.transport = nil
	c.hostBootstrap = hostBootstrapIdle
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	return c.store.SetRuntime(false, "NOT CONNECTED")
}

func (c *Controller) Toggle() (model.Snapshot, error) {
	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	if running {
		return c.Stop(), nil
	}

	return c.Start()
}

func (c *Controller) Status() Status {
	snapshot := c.store.Snapshot()

	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	return Status{
		Running:      running,
		HSMSState:    snapshot.Runtime.HSMSState,
		MessageCount: len(snapshot.Messages),
		ConfigFile:   snapshot.Runtime.ConfigFile,
		Dirty:        snapshot.Runtime.Dirty,
		LastError:    snapshot.Runtime.LastError,
	}
}

func (c *Controller) Inject(message store.InboundMessage) (store.RuntimeResult, error) {
	c.mu.Lock()
	running := c.cancel != nil
	c.mu.Unlock()

	if !running {
		return store.RuntimeResult{}, ErrNotRunning
	}

	now := time.Now().UTC()
	if message.Timestamp.IsZero() {
		message.Timestamp = now
	}

	return c.store.ProcessInbound(message, now), nil
}

func (c *Controller) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			result, err := c.store.RunScheduled(now)
			if err != nil {
				log.Printf("runtime scheduler error: %v", err)
				continue
			}
			c.sendScheduledMessages(result)
		}
	}
}

func (c *Controller) updateRuntimeState(state string) {
	c.mu.Lock()
	running := c.cancel != nil
	if state != "SELECTED" {
		c.hostBootstrap = hostBootstrapIdle
	}
	c.mu.Unlock()

	c.store.SetRuntime(running, state)
	if running && state == "SELECTED" {
		c.beginHostBootstrap()
	}
}

func (c *Controller) currentTransport() *hsms.Manager {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transport
}

func (c *Controller) handleDataMessage(message hsms.Message) ([]hsms.Message, error) {
	now := time.Now().UTC()
	config := c.store.ConfigSnapshot()
	inbound := inboundMessageFromHSMS(message, now)

	if response, ok := autoResponseForMessage(config, message); ok {
		c.store.ProcessInbound(inbound, now)
		c.appendProtocolMessage(now, "OUT", response, "", "")
		return []hsms.Message{response}, nil
	}

	responses := make([]hsms.Message, 0, 2)
	result := c.store.ProcessInbound(inbound, now)
	if response, ok := hostAutoResponseForMessage(config, message); ok {
		responses = append(responses, response)
	}
	for _, response := range responses {
		c.appendProtocolMessage(now, "OUT", response, "", "")
	}
	response, ok, err := c.replyForResult(config, message, result)
	if err != nil {
		return nil, err
	}
	if ok {
		responses = append(responses, response)
	}
	if err := c.advanceHostBootstrap(config, message); err != nil {
		log.Printf("advance host bootstrap after %s: %v", message.Label(), err)
		c.store.SetRuntimeError(normalizeRuntimeError(err))
	}
	if len(responses) == 0 {
		return nil, nil
	}

	return responses, nil
}

func (c *Controller) replyForResult(config model.Snapshot, inbound hsms.Message, result store.RuntimeResult) (hsms.Message, bool, error) {
	if result.Reply == nil {
		return hsms.Message{}, false, nil
	}

	var matchedRule *model.Rule
	for index := range config.Rules {
		if config.Rules[index].ID == result.MatchedRuleID {
			matchedRule = &config.Rules[index]
			break
		}
	}
	if matchedRule == nil {
		return hsms.Message{}, false, errors.New("matched rule not found for outbound reply")
	}

	body := hsms.List(hsms.Binary(byte(matchedRule.Reply.Ack)), hsms.List())
	return hsms.Message{
		SessionID:   uint16(config.HSMS.SessionID),
		Stream:      byte(matchedRule.Reply.Stream),
		Function:    byte(matchedRule.Reply.Function),
		WBit:        false,
		SystemBytes: inbound.SystemBytes,
		Body:        &body,
	}, true, nil
}

func (c *Controller) sendScheduledMessages(result store.RuntimeResult) {
	transport := c.currentTransport()
	if transport == nil {
		return
	}

	config := result.Snapshot
	for _, record := range result.Emitted {
		if record.Detail.Stream != 6 || record.Detail.Function != 11 {
			continue
		}

		message := hsms.BuildS6F11(uint16(config.HSMS.SessionID), 0, record.Label)
		if err := transport.Send(message); err != nil && !errors.Is(err, hsms.ErrNoSelectedSession) {
			log.Printf("send scheduled message %s: %v", record.SF, err)
		}
	}
}

func (c *Controller) appendProtocolMessage(timestamp time.Time, direction string, message hsms.Message, matchedRule string, matchedRuleID string) {
	c.store.AppendProtocolMessage(store.ProtocolMessage{
		Timestamp:     timestamp,
		Direction:     direction,
		Stream:        int(message.Stream),
		Function:      int(message.Function),
		WBit:          message.WBit,
		Label:         message.Label(),
		Body:          message.BodySML(),
		RawSML:        message.RawSML(),
		MatchedRule:   matchedRule,
		MatchedRuleID: matchedRuleID,
	})
}

func inboundMessageFromHSMS(message hsms.Message, timestamp time.Time) store.InboundMessage {
	inbound := store.InboundMessage{
		Timestamp: timestamp,
		Stream:    int(message.Stream),
		Function:  int(message.Function),
		WBit:      message.WBit,
		Label:     message.Label(),
		Body:      message.BodySML(),
		RawSML:    message.RawSML(),
	}

	if rcmd, fields, ok := hsms.ExtractRemoteCommand(message); ok {
		inbound.RCMD = rcmd
		inbound.Fields = fields
	}

	return inbound
}

func autoResponseForMessage(config model.Snapshot, message hsms.Message) (hsms.Message, bool) {
	switch {
	case message.Stream == 1 && message.Function == 13 && config.HSMS.Handshake.AutoS1F13:
		return hsms.BuildS1F14(uint16(config.HSMS.SessionID), message.SystemBytes, config.Device.MDLN, config.Device.SoftRev), true
	case message.Stream == 1 && message.Function == 1 && config.HSMS.Handshake.AutoS1F1:
		return hsms.BuildS1F2(uint16(config.HSMS.SessionID), message.SystemBytes, config.Device.MDLN, config.Device.SoftRev), true
	case message.Stream == 2 && message.Function == 25 && config.HSMS.Handshake.AutoS2F25:
		return hsms.BuildS2F26(uint16(config.HSMS.SessionID), message.SystemBytes, message.Body), true
	default:
		return hsms.Message{}, false
	}
}

func hostAutoResponseForMessage(config model.Snapshot, message hsms.Message) (hsms.Message, bool) {
	if !hostBootstrapEnabled(config.HSMS) {
		return hsms.Message{}, false
	}

	switch {
	case message.Stream == 6 && message.Function == 11 && message.WBit:
		return hsms.BuildS6F12(uint16(config.HSMS.SessionID), message.SystemBytes, 0), true
	default:
		return hsms.Message{}, false
	}
}

func (c *Controller) beginHostBootstrap() {
	config := c.store.ConfigSnapshot()
	if !hostBootstrapEnabled(config.HSMS) {
		return
	}

	c.mu.Lock()
	if c.cancel == nil || c.transport == nil || c.hostBootstrap != hostBootstrapIdle {
		c.mu.Unlock()
		return
	}
	c.hostBootstrap = hostBootstrapAwaitingS1F14
	c.mu.Unlock()

	if err := c.sendStandaloneMessage(hsms.BuildS1F13(uint16(config.HSMS.SessionID), 0)); err != nil {
		c.setHostBootstrap(hostBootstrapIdle)
		log.Printf("start host bootstrap S1F13: %v", err)
		c.store.SetRuntimeError(normalizeRuntimeError(err))
	}
}

func (c *Controller) advanceHostBootstrap(config model.Snapshot, message hsms.Message) error {
	if !hostBootstrapEnabled(config.HSMS) {
		return nil
	}

	switch {
	case message.Stream == 1 && message.Function == 14:
		if !c.transitionHostBootstrap(hostBootstrapAwaitingS1F14, hostBootstrapAwaitingS1F18) {
			return nil
		}
		if err := c.sendStandaloneMessage(hsms.BuildS1F17(uint16(config.HSMS.SessionID), 0)); err != nil {
			c.setHostBootstrap(hostBootstrapIdle)
			return err
		}
	case message.Stream == 1 && message.Function == 18:
		if !c.transitionHostBootstrap(hostBootstrapAwaitingS1F18, hostBootstrapAwaitingS2F32) {
			return nil
		}
		if err := c.sendStandaloneMessage(hsms.BuildS2F31(uint16(config.HSMS.SessionID), 0, time.Now())); err != nil {
			c.setHostBootstrap(hostBootstrapIdle)
			return err
		}
	case message.Stream == 2 && message.Function == 32:
		if c.transitionHostBootstrap(hostBootstrapAwaitingS2F32, hostBootstrapReady) {
			return nil
		}
	}

	return nil
}

func (c *Controller) sendStandaloneMessage(message hsms.Message) error {
	transport := c.currentTransport()
	if transport == nil {
		return ErrNotRunning
	}
	if err := transport.Send(message); err != nil {
		return err
	}

	c.appendProtocolMessage(time.Now().UTC(), "OUT", message, "", "")
	return nil
}

func (c *Controller) transitionHostBootstrap(expect hostBootstrapPhase, next hostBootstrapPhase) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hostBootstrap != expect {
		return false
	}
	c.hostBootstrap = next
	return true
}

func (c *Controller) setHostBootstrap(phase hostBootstrapPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hostBootstrap = phase
}

func hostBootstrapEnabled(config model.HsmsConfig) bool {
	return strings.EqualFold(strings.TrimSpace(config.Mode), "active") && config.Handshake.AutoHostStartup
}

func normalizeRuntimeError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, io.EOF) {
		return "connection closed by peer"
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "unknown runtime error"
	}

	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "connection refused"):
		return "connection refused"
	case strings.Contains(lower, "broken pipe"),
		strings.Contains(lower, "connection reset"),
		strings.Contains(lower, "reset by peer"),
		strings.Contains(lower, "forcibly closed"),
		strings.Contains(lower, "use of closed network connection"):
		return "connection dropped"
	case strings.Contains(lower, "i/o timeout"):
		return "transport timeout"
	default:
		return message
	}
}
