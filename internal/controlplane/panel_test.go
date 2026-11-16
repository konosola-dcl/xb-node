package controlplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cedar2025/xboard-node/internal/config"
	panelapi "github.com/cedar2025/xboard-node/internal/panel"
)

func TestPanelControlPlaneInitialRejectsInvalidCustomOutbounds(t *testing.T) {
	server := newPanelTestServer(`{"protocol":"shadowsocks","server_port":8388,"custom_outbounds":[{"tag":"proxy","protocol":"socks","proxy_tag":"missing","settings":{"server":"2.2.2.2","server_port":1080}}]}`)
	defer server.Close()

	cp := NewPanelControlPlane(config.PanelConfig{URL: server.URL, Token: "token", NodeID: 1}, config.WSConfig{}, config.KernelConfig{Type: "singbox"})
	_, err := cp.Initial(context.Background(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `initial config normalize: validate custom outbounds: custom_outbounds[0].proxy_tag references unknown outbound "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPanelControlPlanePollRejectsInvalidCustomOutbounds(t *testing.T) {
	server := newPanelTestServer(`{"protocol":"shadowsocks","server_port":8388,"custom_outbounds":[{"tag":"proxy","protocol":"socks","proxy_tag":"missing","settings":{"server":"2.2.2.2","server_port":1080}}]}`)
	defer server.Close()

	cp := NewPanelControlPlane(config.PanelConfig{URL: server.URL, Token: "token", NodeID: 1}, config.WSConfig{}, config.KernelConfig{Type: "singbox"})
	_, err := cp.Poll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `poll config normalize: validate custom outbounds: custom_outbounds[0].proxy_tag references unknown outbound "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTranslateWSEventRejectsInvalidCustomOutbounds(t *testing.T) {
	_, err := TranslateWSEvent(panelapi.WSEvent{
		Type: panelapi.WSEventSyncConfig,
		Config: &panelapi.NodeConfig{
			Protocol:   "shadowsocks",
			ServerPort: 8388,
			CustomOutbounds: []panelapi.OutboundConfig{
				{Tag: "proxy", Protocol: "socks", ProxyTag: "missing", Settings: map[string]any{"server": "2.2.2.2", "server_port": 1080}},
			},
		},
	}, config.KernelConfig{Type: "singbox"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `translate node config: validate custom outbounds: custom_outbounds[0].proxy_tag references unknown outbound "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPanelControlPlaneWSDisableForcesREST(t *testing.T) {
	server := newPanelTestServerWithHandshake(
		`{"websocket":{"enabled":true,"ws_url":"ws://127.0.0.1:1/ws"},"settings":{"push_interval":60,"pull_interval":60}}`,
		`{"protocol":"shadowsocks","server_port":8388,"cipher":"aes-128-gcm"}`,
	)
	defer server.Close()
	panelCfg := config.PanelConfig{URL: server.URL, Token: "token", NodeID: 1}
	kcfg := config.KernelConfig{Type: "singbox"}

	// Control: with ws.disable unset, this panel hands out a push client, so
	// the assertions below are not passing vacuously.
	enabled, err := NewPanelControlPlane(panelCfg, config.WSConfig{}, kcfg).Initial(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("Initial (ws enabled): %v", err)
	}
	if enabled.Push == nil {
		t.Fatal("expected a push client when the panel enables WS and ws.disable is unset")
	}

	disable := true
	cp := NewPanelControlPlane(panelCfg, config.WSConfig{Disable: &disable}, kcfg)
	bootstrap, err := cp.Initial(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("Initial (ws.disable): %v", err)
	}
	if bootstrap.Push != nil {
		t.Error("ws.disable: Initial returned a push client")
	}
	if bootstrap.Config == nil || len(bootstrap.Users) != 1 {
		t.Errorf("ws.disable: expected config and users fetched over REST, got config=%v users=%d", bootstrap.Config != nil, len(bootstrap.Users))
	}
	if cp.SupportsDiscovery() {
		t.Error("ws.disable: SupportsDiscovery should be false so the service never re-enables WS")
	}
	if push, err := cp.Discover(context.Background(), nil, nil, nil); push != nil || err != nil {
		t.Errorf("ws.disable: Discover = (%v, %v), want (nil, nil)", push, err)
	}
}

func newPanelTestServer(configBody string) *httptest.Server {
	return newPanelTestServerWithHandshake(`{"websocket":{"enabled":false},"settings":{"push_interval":60,"pull_interval":60}}`, configBody)
}

func newPanelTestServerWithHandshake(handshakeBody, configBody string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/server/handshake", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(handshakeBody))
	})
	mux.HandleFunc("/api/v1/server/UniProxy/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(configBody))
	})
	mux.HandleFunc("/api/v1/server/UniProxy/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"id":1,"uuid":"11111111-1111-1111-1111-111111111111"}]}`))
	})
	return httptest.NewServer(mux)
}

func TestTranslateWSEventRejectsUnsupportedProtocolForKernel(t *testing.T) {
	_, err := TranslateWSEvent(panelapi.WSEvent{
		Type: panelapi.WSEventSyncConfig,
		Config: &panelapi.NodeConfig{
			Protocol:   "shadowsocks",
			ServerPort: 8388,
			CustomOutbounds: []panelapi.OutboundConfig{
				{Tag: "hy2", Protocol: "hysteria2", Settings: map[string]any{"server": "2.2.2.2", "server_port": 8443}},
			},
		},
	}, config.KernelConfig{Type: "xray"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `translate node config: validate custom outbounds: custom_outbounds[0].protocol "hysteria2" is not supported by kernel "xray"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
