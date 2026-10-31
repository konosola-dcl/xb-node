package singbox

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/kernel"
	"github.com/cedar2025/xboard-node/internal/model"
)

const (
	rotOldUUID    = "11111111-1111-1111-1111-111111111111"
	rotNewUUID    = "35b2af08-9507-4ebd-99fc-e4172286c88e"
	rotStableUUID = "22222222-2222-2222-2222-222222222222"
)

// kernelLog collects sing-box's own log output, which is the only place an
// inbound reports which UUIDs it rejected.
type kernelLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *kernelLog) contains(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.Contains(line, s) {
			return true
		}
	}
	return false
}

// captureKernelLog collects the log of any sing-box instance created inside
// fn, since sing-box binds its logger to os.Stderr at creation.
func captureKernelLog(t *testing.T, fn func()) *kernelLog {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	logs := &kernelLog{}
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			logs.mu.Lock()
			logs.lines = append(logs.lines, sc.Text())
			logs.mu.Unlock()
		}
	}()
	stderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = stderr }()
	fn()
	return logs
}

func startVLESSSingBox(t *testing.T, users []model.UserSpec) (*SingBox, int, *kernelLog) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	s := New(config.KernelConfig{Type: "singbox", LogLevel: "error"})
	nc := &model.NodeSpec{Protocol: "vless", ServerPort: port, Network: "tcp"}
	logs := captureKernelLog(t, func() {
		err = s.Start(nc, users, kernel.TLSCert{})
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(s.Stop)
	return s, port, logs
}

// vlessAccepted sends a VLESS request with uuid and waits for the server to
// close it. Unknown UUIDs are logged by the inbound as "unknown UUID"; known
// ones get routed and hit the block outbound for the private target.
func vlessAccepted(t *testing.T, port int, logs *kernelLog, uuid string) bool {
	t.Helper()
	id, err := hex.DecodeString(strings.ReplaceAll(uuid, "-", ""))
	if err != nil {
		t.Fatalf("decode uuid: %v", err)
	}
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// version, uuid, addons length, command TCP, port 9, IPv4 127.0.0.1, payload
	req := []byte{0}
	req = append(req, id...)
	req = append(req, 0, 1)
	req = binary.BigEndian.AppendUint16(req, 9)
	req = append(req, 1, 127, 0, 0, 1)
	req = append(req, "ping"...)
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if logs.contains("unknown UUID: " + uuid) {
			return false
		}
		if logs.contains("127.0.0.1:9 using outbound/block") {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no verdict from sing-box for %s", uuid)
	return false
}

func TestSingBoxAddUsersAppliesUUIDRotation(t *testing.T) {
	// Delta add carrying a new UUID for a user the kernel already has.
	s, port, logs := startVLESSSingBox(t, []model.UserSpec{{ID: 1, UUID: rotOldUUID}, {ID: 2, UUID: rotStableUUID}})

	added, err := s.AddUsers([]model.UserSpec{{ID: 1, UUID: rotNewUUID}})
	if err != nil {
		t.Fatalf("AddUsers: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if vlessAccepted(t, port, logs, rotOldUUID) {
		t.Fatal("old UUID still accepted after AddUsers")
	}
	if !vlessAccepted(t, port, logs, rotNewUUID) {
		t.Fatal("new UUID rejected after AddUsers")
	}
}

func TestSingBoxReloadRestartsOnSamePort(t *testing.T) {
	// A changed config restarts the instance, so the old one has to release
	// the port before the new one binds it, and the tracker has to survive so
	// traffic not yet reported is kept.
	s, port, _ := startVLESSSingBox(t, []model.UserSpec{{ID: 1, UUID: rotOldUUID}})
	tracker := s.connTracker

	nc := &model.NodeSpec{Protocol: "vless", ServerPort: port, Network: "tcp"}
	var err error
	logs := captureKernelLog(t, func() {
		err = s.Reload(nc, []model.UserSpec{{ID: 1, UUID: rotNewUUID}}, kernel.TLSCert{})
	})
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if s.connTracker != tracker {
		t.Fatal("Reload replaced the connTracker")
	}
	if vlessAccepted(t, port, logs, rotOldUUID) {
		t.Fatal("old UUID still accepted after Reload")
	}
	if !vlessAccepted(t, port, logs, rotNewUUID) {
		t.Fatal("new UUID rejected after Reload")
	}
}
