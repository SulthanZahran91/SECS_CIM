package hsms

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"secsim/design/backend/internal/model"
)

var ErrNoSelectedSession = errors.New("no selected HSMS session")

type Handlers struct {
	OnData        func(Message) ([]Message, error)
	OnStateChange func(string)
	OnError       func(error)
}

type Manager struct {
	config          model.HsmsConfig
	handlers        Handlers
	mu              sync.RWMutex
	listener        net.Listener
	current         *session
	nextSystemBytes atomic.Uint32
	started         atomic.Bool
}

type session struct {
	conn      net.Conn
	writeCh   chan *Frame
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	selected  bool
}

func NewManager(config model.HsmsConfig, handlers Handlers) *Manager {
	manager := &Manager{
		config:   config,
		handlers: handlers,
	}
	manager.nextSystemBytes.Store(0)
	return manager
}

func (m *Manager) Start(ctx context.Context) error {
	if !m.started.CompareAndSwap(false, true) {
		return nil
	}

	go func() {
		<-ctx.Done()
		m.shutdown()
	}()

	if isActiveMode(m.config.Mode) {
		m.publishState("CONNECTING")
		go m.activeLoop(ctx)
		return nil
	}

	listener, err := net.Listen("tcp", joinAddress(m.config.IP, m.config.Port, false))
	if err != nil {
		m.started.Store(false)
		return err
	}

	m.mu.Lock()
	m.listener = listener
	m.mu.Unlock()

	m.publishState("LISTENING")
	go m.acceptLoop(ctx, listener)
	return nil
}

func (m *Manager) Send(message Message) error {
	session := m.currentSession()
	if session == nil || !session.isSelected() {
		return ErrNoSelectedSession
	}

	if message.SessionID == 0 {
		message.SessionID = uint16(m.config.SessionID)
	}
	if message.SystemBytes == 0 {
		message.SystemBytes = m.nextSystemByte()
	}

	return m.sendOnSession(session, message)
}

func (m *Manager) acceptLoop(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if m.handlers.OnError != nil {
				m.handlers.OnError(err)
			}
			continue
		}

		if m.currentSession() != nil {
			_ = conn.Close()
			continue
		}

		activeSession := newSession(conn)
		m.setCurrentSession(activeSession)
		m.publishState("CONNECTED")

		go m.runSession(ctx, activeSession, false)
	}
}

func (m *Manager) activeLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", joinAddress(m.config.IP, m.config.Port, true))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if m.handlers.OnError != nil {
				m.handlers.OnError(err)
			}
			if !sleepContext(ctx, timerDuration(m.config.Timers.T5, time.Second)) {
				return
			}
			continue
		}

		activeSession := newSession(conn)
		m.setCurrentSession(activeSession)
		m.publishState("CONNECTED")

		if err := activeSession.send(NewControlFrame(uint16(m.config.SessionID), m.nextSystemByte(), STypeSelectReq, 0)); err != nil {
			activeSession.close()
			m.clearCurrentSession(activeSession)
			if !sleepContext(ctx, timerDuration(m.config.Timers.T5, time.Second)) {
				return
			}
			continue
		}

		m.armSelectionTimeout(activeSession, timerDuration(m.config.Timers.T6, 5*time.Second))
		m.runSession(ctx, activeSession, true)

		if !sleepContext(ctx, timerDuration(m.config.Timers.T5, time.Second)) {
			return
		}
		m.publishState("CONNECTING")
	}
}

func (m *Manager) runSession(ctx context.Context, session *session, active bool) {
	defer func() {
		session.close()
		m.clearCurrentSession(session)

		if ctx.Err() != nil {
			return
		}
		if active {
			m.publishState("CONNECTING")
			return
		}
		m.publishState("LISTENING")
	}()

	if !active {
		m.armSelectionTimeout(session, timerDuration(m.config.Timers.T7, 10*time.Second))
	}

	for {
		if err := session.conn.SetReadDeadline(time.Now().Add(timerDuration(m.config.Timers.T8, 5*time.Second))); err != nil && m.handlers.OnError != nil {
			m.handlers.OnError(err)
		}

		frame, err := ReadFrame(session.conn)
		if err != nil {
			if ctx.Err() == nil && !isClosedNetworkError(err) && m.handlers.OnError != nil {
				m.handlers.OnError(err)
			}
			return
		}

		if frame.SType != STypeData {
			if err := m.handleControlFrame(session, frame, active); err != nil {
				if ctx.Err() == nil && m.handlers.OnError != nil {
					m.handlers.OnError(err)
				}
				return
			}
			continue
		}

		message, err := DecodeMessage(frame)
		if err != nil {
			if m.handlers.OnError != nil {
				m.handlers.OnError(err)
			}
			continue
		}

		if m.handlers.OnData == nil {
			continue
		}

		responses, err := m.handlers.OnData(message)
		if err != nil {
			if m.handlers.OnError != nil {
				m.handlers.OnError(err)
			}
			continue
		}

		for _, response := range responses {
			if response.SessionID == 0 {
				response.SessionID = frame.SessionID
			}
			if response.SystemBytes == 0 {
				response.SystemBytes = frame.SystemBytes
			}
			if err := m.sendOnSession(session, response); err != nil {
				if ctx.Err() == nil && m.handlers.OnError != nil {
					m.handlers.OnError(err)
				}
				return
			}
		}
	}
}

func (m *Manager) handleControlFrame(session *session, frame *Frame, active bool) error {
	switch frame.SType {
	case STypeSelectReq:
		if err := session.send(NewControlFrame(uint16(m.config.SessionID), frame.SystemBytes, STypeSelectRsp, SelectStatusSuccess)); err != nil {
			return err
		}
		session.setSelected(true)
		m.publishState("SELECTED")
		return nil
	case STypeSelectRsp:
		if !active {
			return nil
		}
		if frame.ControlCode != SelectStatusSuccess {
			return fmt.Errorf("select response rejected with status %d", frame.ControlCode)
		}
		session.setSelected(true)
		m.publishState("SELECTED")
		return nil
	case STypeDeselectReq:
		session.setSelected(false)
		m.publishState("CONNECTED")
		return session.send(NewControlFrame(uint16(m.config.SessionID), frame.SystemBytes, STypeDeselectRsp, 0))
	case STypeDeselectRsp:
		session.setSelected(false)
		m.publishState("CONNECTED")
		return nil
	case STypeLinktestReq:
		return session.send(NewControlFrame(uint16(m.config.SessionID), frame.SystemBytes, STypeLinktestRsp, 0))
	case STypeLinktestRsp:
		return nil
	case STypeSeparateReq:
		session.close()
		return nil
	default:
		return fmt.Errorf("unsupported control message stype %d", frame.SType)
	}
}

func (m *Manager) sendOnSession(session *session, message Message) error {
	frame, err := EncodeMessage(message)
	if err != nil {
		return err
	}
	return session.send(frame)
}

func (m *Manager) shutdown() {
	m.mu.Lock()
	listener := m.listener
	m.listener = nil
	current := m.current
	m.current = nil
	m.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	if current != nil {
		current.close()
	}

	m.publishState("NOT CONNECTED")
	m.started.Store(false)
}

func (m *Manager) currentSession() *session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *Manager) setCurrentSession(session *session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = session
}

func (m *Manager) clearCurrentSession(session *session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == session {
		m.current = nil
	}
}

func (m *Manager) publishState(state string) {
	if m.handlers.OnStateChange != nil {
		m.handlers.OnStateChange(state)
	}
}

func (m *Manager) nextSystemByte() uint32 {
	return m.nextSystemBytes.Add(1)
}

func (m *Manager) armSelectionTimeout(session *session, timeout time.Duration) {
	if timeout <= 0 {
		return
	}

	time.AfterFunc(timeout, func() {
		if !session.isSelected() {
			session.close()
		}
	})
}

func joinAddress(host string, port int, active bool) string {
	if host == "" {
		if active {
			host = "127.0.0.1"
		} else {
			host = "0.0.0.0"
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func timerDuration(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func isActiveMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "active")
}

func isClosedNetworkError(err error) bool {
	return errors.Is(err, net.ErrClosed) || strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func newSession(conn net.Conn) *session {
	activeSession := &session{
		conn:    conn,
		writeCh: make(chan *Frame, 16),
		done:    make(chan struct{}),
	}
	go activeSession.writeLoop()
	return activeSession
}

func (s *session) writeLoop() {
	for {
		select {
		case <-s.done:
			return
		case frame := <-s.writeCh:
			if frame == nil {
				continue
			}
			if err := WriteFrame(s.conn, frame); err != nil {
				s.close()
				return
			}
		}
	}
}

func (s *session) send(frame *Frame) error {
	select {
	case <-s.done:
		return net.ErrClosed
	case s.writeCh <- frame:
		return nil
	}
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.conn.Close()
	})
}

func (s *session) setSelected(selected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selected = selected
}

func (s *session) isSelected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selected
}
