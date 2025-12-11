package alert

import (
	"net/http"
	"testing"
)

type mockHTTPClient struct {
	statusCode int
	err        error
	lastReq    *http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       http.NoBody,
	}, nil
}

func TestNewManager(t *testing.T) {
	m := NewManager(true, "https://hooks.slack.com/test")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if !m.enabled {
		t.Error("expected enabled to be true")
	}
	if m.slackWebhook != "https://hooks.slack.com/test" {
		t.Error("expected slack webhook to be set")
	}
}

func TestSendTamperAlert_Disabled(t *testing.T) {
	m := NewManager(false, "https://hooks.slack.com/test")
	err := m.SendTamperAlert("users", "UPDATE", "123", "unauthorized update")
	if err != nil {
		t.Errorf("expected nil error when disabled, got: %v", err)
	}
}

func TestSendTamperAlert_EmptyWebhook(t *testing.T) {
	m := NewManager(true, "")
	err := m.SendTamperAlert("users", "UPDATE", "123", "unauthorized update")
	if err != nil {
		t.Errorf("expected nil error with empty webhook, got: %v", err)
	}
}

func TestSendTamperAlert_Success(t *testing.T) {
	mock := &mockHTTPClient{statusCode: http.StatusOK}
	m := NewManagerWithClient(true, "https://hooks.slack.com/test", mock)

	err := m.SendTamperAlert("audit_logs", "UPDATE", "456", "unauthorized modification")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if mock.lastReq == nil {
		t.Fatal("expected request to be made")
	}
	if mock.lastReq.Method != http.MethodPost {
		t.Errorf("expected POST method, got: %s", mock.lastReq.Method)
	}
	if mock.lastReq.Header.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type to be application/json")
	}
}

func TestSendTamperAlert_SlackError(t *testing.T) {
	mock := &mockHTTPClient{statusCode: http.StatusInternalServerError}
	m := NewManagerWithClient(true, "https://hooks.slack.com/test", mock)

	err := m.SendTamperAlert("audit_logs", "DELETE", "789", "unauthorized deletion")
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestSendHashChainBrokenAlert_Success(t *testing.T) {
	mock := &mockHTTPClient{statusCode: http.StatusOK}
	m := NewManagerWithClient(true, "https://hooks.slack.com/test", mock)

	err := m.SendHashChainBrokenAlert("audit_logs", 42, "abc123", "xyz789")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if mock.lastReq == nil {
		t.Fatal("expected request to be made")
	}
}

func TestSendHashChainBrokenAlert_Disabled(t *testing.T) {
	m := NewManager(false, "https://hooks.slack.com/test")
	err := m.SendHashChainBrokenAlert("audit_logs", 42, "abc123", "xyz789")
	if err != nil {
		t.Errorf("expected nil error when disabled, got: %v", err)
	}
}
