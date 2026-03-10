package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"secsim/design/backend/internal/hsms"
	"secsim/design/backend/internal/model"
)

type configResponse struct {
	HSMS model.HsmsConfig `json:"hsms"`
}

func main() {
	apiBase := flag.String("api", "http://127.0.0.1:8080", "SECSIM HTTP API base URL")
	hsmsHost := flag.String("host", "", "HSMS host override; defaults to config.hsms.ip or 127.0.0.1")
	carrierID := flag.String("carrier", "CARR001", "Carrier ID for the default TRANSFER smoke test")
	sourcePort := flag.String("source", "LP01", "Source port for the default TRANSFER smoke test")
	timeout := flag.Duration("timeout", 3*time.Second, "Network timeout per step")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	config, err := fetchConfig(client, *apiBase)
	if err != nil {
		log.Fatalf("fetch config: %v", err)
	}
	if !strings.EqualFold(strings.TrimSpace(config.HSMS.Mode), "passive") {
		log.Fatalf("smoke test expects passive HSMS mode, got %q", config.HSMS.Mode)
	}

	if err := startSimulator(client, *apiBase); err != nil {
		log.Fatalf("start simulator: %v", err)
	}

	host := strings.TrimSpace(*hsmsHost)
	if host == "" {
		host = strings.TrimSpace(config.HSMS.IP)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", config.HSMS.Port))
	conn, err := net.DialTimeout("tcp", addr, *timeout)
	if err != nil {
		log.Fatalf("connect HSMS %s: %v", addr, err)
	}
	defer conn.Close()

	fmt.Printf("Connected to HSMS at %s\n", addr)

	setDeadline(conn, *timeout)
	if err := hsms.WriteFrame(conn, hsms.NewControlFrame(uint16(config.HSMS.SessionID), 0x1001, hsms.STypeSelectReq, 0)); err != nil {
		log.Fatalf("send Select.req: %v", err)
	}
	selectRsp := mustReadFrame(conn)
	if selectRsp.SType != hsms.STypeSelectRsp || selectRsp.ControlCode != hsms.SelectStatusSuccess {
		log.Fatalf("unexpected Select.rsp: stype=%d control=%d", selectRsp.SType, selectRsp.ControlCode)
	}
	fmt.Println("HSMS select succeeded")

	writeMessage(conn, hsms.Message{
		SessionID:   uint16(config.HSMS.SessionID),
		Stream:      1,
		Function:    13,
		WBit:        true,
		SystemBytes: 0x1002,
		Body:        itemPtr(hsms.List()),
	}, *timeout)
	s1f14 := mustReadMessage(conn, *timeout)
	expectData(s1f14, 1, 14)
	fmt.Printf("Received %s: %s\n", s1f14.Label(), s1f14.RawSML())

	writeMessage(conn, hsms.Message{
		SessionID:   uint16(config.HSMS.SessionID),
		Stream:      2,
		Function:    41,
		WBit:        true,
		SystemBytes: 0x1003,
		Body: itemPtr(hsms.List(
			hsms.ASCII("TRANSFER"),
			hsms.List(
				hsms.List(hsms.ASCII("CarrierID"), hsms.ASCII(*carrierID)),
				hsms.List(hsms.ASCII("SourcePort"), hsms.ASCII(*sourcePort)),
			),
		)),
	}, *timeout)
	s2f42 := mustReadMessage(conn, *timeout)
	expectData(s2f42, 2, 42)
	fmt.Printf("Received %s: %s\n", s2f42.Label(), s2f42.RawSML())

	for index := 0; index < 2; index++ {
		event := mustReadMessage(conn, *timeout)
		expectData(event, 6, 11)
		fmt.Printf("Received event: %s\n", event.RawSML())
	}

	fmt.Println("Smoke test completed successfully")
}

func fetchConfig(client *http.Client, apiBase string) (configResponse, error) {
	endpoint, err := joinURL(apiBase, "/api/config")
	if err != nil {
		return configResponse{}, err
	}

	response, err := client.Get(endpoint)
	if err != nil {
		return configResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return configResponse{}, fmt.Errorf("unexpected status %d", response.StatusCode)
	}

	var config configResponse
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		return configResponse{}, err
	}
	return config, nil
}

func startSimulator(client *http.Client, apiBase string) error {
	endpoint, err := joinURL(apiBase, "/api/sim/start")
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	return nil
}

func joinURL(base string, path string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}

func writeMessage(conn net.Conn, message hsms.Message, timeout time.Duration) {
	frame, err := hsms.EncodeMessage(message)
	if err != nil {
		log.Fatalf("encode message %s: %v", message.RawSML(), err)
	}
	setDeadline(conn, timeout)
	if err := hsms.WriteFrame(conn, frame); err != nil {
		log.Fatalf("write message %s: %v", message.RawSML(), err)
	}
}

func mustReadFrame(conn net.Conn) *hsms.Frame {
	frame, err := hsms.ReadFrame(conn)
	if err != nil {
		log.Fatalf("read frame: %v", err)
	}
	return frame
}

func mustReadMessage(conn net.Conn, timeout time.Duration) hsms.Message {
	setDeadline(conn, timeout)
	frame := mustReadFrame(conn)
	if frame.SType != hsms.STypeData {
		log.Fatalf("expected data frame, got stype=%d", frame.SType)
	}
	message, err := hsms.DecodeMessage(frame)
	if err != nil {
		log.Fatalf("decode data frame: %v", err)
	}
	return message
}

func expectData(message hsms.Message, stream byte, function byte) {
	if message.Stream != stream || message.Function != function {
		log.Fatalf("expected S%dF%d, got %s", stream, function, message.RawSML())
	}
}

func itemPtr(item hsms.Item) *hsms.Item {
	return &item
}

func setDeadline(conn net.Conn, timeout time.Duration) {
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		fmt.Fprintf(os.Stderr, "set deadline: %v\n", err)
	}
}
