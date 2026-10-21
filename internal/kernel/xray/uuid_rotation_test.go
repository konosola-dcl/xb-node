package xray

import (
	"context"
	"net"
	"testing"

	"github.com/cedar2025/xboard-node/internal/config"
	"github.com/cedar2025/xboard-node/internal/kernel"
	"github.com/cedar2025/xboard-node/internal/model"
	"github.com/xtls/xray-core/proxy/vless"
)

const (
	rotOldUUID    = "11111111-1111-1111-1111-111111111111"
	rotNewUUID    = "35b2af08-9507-4ebd-99fc-e4172286c88e"
	rotStableUUID = "22222222-2222-2222-2222-222222222222"
)

// startVLESSXray runs a real xray instance with a plain VLESS inbound so the
// tests go through xray's own UserManager.
func startVLESSXray(t *testing.T, users []model.UserSpec) *Xray {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	x := New(config.KernelConfig{Type: "xray", LogLevel: "warning"})
	nc := &model.NodeSpec{Protocol: "vless", ServerPort: port, Network: "tcp"}
	if err := x.Start(nc, users, kernel.TLSCert{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(x.Stop)
	return x
}

// kernelUUID returns the UUID xray currently accepts for the user, or "".
func kernelUUID(t *testing.T, x *Xray, userID int) string {
	t.Helper()
	um, err := x.getUserManager()
	if err != nil {
		t.Fatalf("getUserManager: %v", err)
	}
	mu := um.GetUser(context.Background(), userEmail(userID))
	if mu == nil {
		return ""
	}
	return mu.Account.(*vless.MemoryAccount).ID.String()
}

func TestXrayUpdateUsersAppliesUUIDRotation(t *testing.T) {
	// Full sync (REST poll / machine mode) after a subscription reset.
	x := startVLESSXray(t, []model.UserSpec{{ID: 1, UUID: rotOldUUID}, {ID: 2, UUID: rotStableUUID}})

	if _, _, err := x.UpdateUsers([]model.UserSpec{{ID: 1, UUID: rotNewUUID}, {ID: 2, UUID: rotStableUUID}}); err != nil {
		t.Fatalf("UpdateUsers: %v", err)
	}

	if got := kernelUUID(t, x, 1); got != rotNewUUID {
		t.Fatalf("user 1 UUID in kernel = %q, want %q", got, rotNewUUID)
	}
	if got := kernelUUID(t, x, 2); got != rotStableUUID {
		t.Fatalf("unchanged user UUID in kernel = %q, want %q", got, rotStableUUID)
	}
}

func TestXrayAddUsersAppliesUUIDRotation(t *testing.T) {
	// Delta add carrying a new UUID for a user the kernel already has.
	x := startVLESSXray(t, []model.UserSpec{{ID: 1, UUID: rotOldUUID}, {ID: 2, UUID: rotStableUUID}})

	added, err := x.AddUsers([]model.UserSpec{{ID: 1, UUID: rotNewUUID}})
	if err != nil {
		t.Fatalf("AddUsers: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if got := kernelUUID(t, x, 1); got != rotNewUUID {
		t.Fatalf("user 1 UUID in kernel = %q, want %q", got, rotNewUUID)
	}
	if got := kernelUUID(t, x, 2); got != rotStableUUID {
		t.Fatalf("unchanged user UUID in kernel = %q, want %q", got, rotStableUUID)
	}
}
