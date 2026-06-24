package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendRawNotificationUsesProxy(t *testing.T) {
	targetHits := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case targetHits <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusTeapot)
	}))
	defer target.Close()

	type proxyRequest struct {
		Method      string
		URL         string
		ContentType string
		Body        string
	}
	proxyRequests := make(chan proxyRequest, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		proxyRequests <- proxyRequest{
			Method:      r.Method,
			URL:         r.URL.String(),
			ContentType: r.Header.Get("Content-Type"),
			Body:        string(body),
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()

	err := sendRawNotification(http.MethodPost, target.URL+"/notify", "application/json", []byte(`{"ok":true}`), "", proxy.URL)
	if err != nil {
		t.Fatalf("sendRawNotification error = %v", err)
	}

	select {
	case got := <-proxyRequests:
		if got.Method != http.MethodPost {
			t.Fatalf("proxy method = %s, want POST", got.Method)
		}
		if got.URL != target.URL+"/notify" {
			t.Fatalf("proxy URL = %s, want %s", got.URL, target.URL+"/notify")
		}
		if got.ContentType != "application/json" {
			t.Fatalf("proxy Content-Type = %s, want application/json", got.ContentType)
		}
		if got.Body != `{"ok":true}` {
			t.Fatalf("proxy body = %s, want payload", got.Body)
		}
	default:
		t.Fatal("proxy was not used")
	}

	select {
	case <-targetHits:
		t.Fatal("target server was called directly")
	default:
	}
}

func TestValidateNotificationChannelRejectsInvalidProxy(t *testing.T) {
	item := notificationChannel{
		Name:     "webhook",
		Type:     "webhook",
		URL:      "https://example.com/hook",
		ProxyURL: "ftp://proxy.example.com:21",
		Enabled:  true,
	}
	err := validateNotificationChannel(item)
	if err == nil || !strings.Contains(err.Error(), "代理地址仅支持") {
		t.Fatalf("validateNotificationChannel error = %v, want unsupported proxy scheme", err)
	}

	item.ProxyURL = "://bad"
	err = validateNotificationChannel(item)
	if err == nil || !strings.Contains(err.Error(), "代理地址格式不正确") {
		t.Fatalf("validateNotificationChannel error = %v, want invalid proxy URL", err)
	}
}
