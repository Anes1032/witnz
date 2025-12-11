package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Manager struct {
	enabled      bool
	slackWebhook string
	httpClient   HTTPClient
}

type slackMessage struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Title  string       `json:"title"`
	Fields []slackField `json:"fields"`
	Footer string       `json:"footer"`
	Ts     int64        `json:"ts"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func NewManager(enabled bool, slackWebhook string) *Manager {
	return &Manager{
		enabled:      enabled,
		slackWebhook: slackWebhook,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func NewManagerWithClient(enabled bool, slackWebhook string, client HTTPClient) *Manager {
	return &Manager{
		enabled:      enabled,
		slackWebhook: slackWebhook,
		httpClient:   client,
	}
}

func (m *Manager) SendTamperAlert(tableName, operation, recordID, details string) error {
	if !m.enabled || m.slackWebhook == "" {
		return nil
	}

	msg := slackMessage{
		Text: "🚨 *TAMPERING DETECTED*",
		Attachments: []slackAttachment{
			{
				Color: "danger",
				Title: "Database Tampering Alert",
				Fields: []slackField{
					{Title: "Table", Value: tableName, Short: true},
					{Title: "Operation", Value: operation, Short: true},
					{Title: "Record ID", Value: recordID, Short: true},
					{Title: "Details", Value: details, Short: false},
				},
				Footer: "Witnz Tamper Detection",
				Ts:     time.Now().Unix(),
			},
		},
	}

	return m.sendSlackMessage(msg)
}

func (m *Manager) SendHashChainBrokenAlert(tableName string, sequenceNum uint64, expectedHash, actualHash string) error {
	if !m.enabled || m.slackWebhook == "" {
		return nil
	}

	msg := slackMessage{
		Text: "🚨 *HASH CHAIN INTEGRITY VIOLATION*",
		Attachments: []slackAttachment{
			{
				Color: "danger",
				Title: "Hash Chain Broken",
				Fields: []slackField{
					{Title: "Table", Value: tableName, Short: true},
					{Title: "Sequence", Value: fmt.Sprintf("%d", sequenceNum), Short: true},
					{Title: "Expected Hash", Value: expectedHash, Short: false},
					{Title: "Actual Hash", Value: actualHash, Short: false},
				},
				Footer: "Witnz Tamper Detection",
				Ts:     time.Now().Unix(),
			},
		},
	}

	return m.sendSlackMessage(msg)
}

func (m *Manager) SendSystemAlert(title, message, severity string) error {
	if !m.enabled || m.slackWebhook == "" {
		return nil
	}

	color := "danger"
	if severity == "warning" {
		color = "warning"
	} else if severity == "good" {
		color = "good"
	}

	msg := slackMessage{
		Text: fmt.Sprintf("🚨 *SYSTEM ALERT: %s*", title),
		Attachments: []slackAttachment{
			{
				Color: color,
				Title: title,
				Fields: []slackField{
					{Title: "Message", Value: message, Short: false},
				},
				Footer: "Witnz System Monitor",
				Ts:     time.Now().Unix(),
			},
		},
	}

	return m.sendSlackMessage(msg)
}

func (m *Manager) sendSlackMessage(msg slackMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, m.slackWebhook, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}
