package team

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBrokerPersistFlushesStateFile pins the "persisting broker state" half
// of the shutdown contract: Persist writes broker-state.json even when the
// file is absent (a fresh broker that never mutated anything), through the
// same atomic write every mutation uses.
func TestBrokerPersistFlushesStateFile(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "broker-state.json")
	b := NewBrokerAt(statePath)
	// The constructor only LOADS state; drop anything seeded so the test
	// observes Persist's write rather than a leftover.
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove pre-existing state file: %v", err)
	}
	if err := b.Persist(); err != nil {
		t.Fatalf("Persist() = %v, want nil", err)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file not written by Persist: %v", err)
	}
	if len(data) == 0 || data[0] != '{' {
		t.Fatalf("state file does not look like JSON state: %q", string(data[:min(len(data), 40)]))
	}
}

// TestCloseListenersClosesBrokerAndWebUIListeners pins the "close listeners"
// half: after StartOnPort + ServeWebUI both ports accept, and after
// CloseListeners neither does. Pre-fix, the web UI listener was a
// function-local variable nothing could reach — only process exit released
// the port.
func TestCloseListenersClosesBrokerAndWebUIListeners(t *testing.T) {
	b := NewBrokerAt(filepath.Join(t.TempDir(), "broker-state.json"))
	if err := b.StartOnPort(0); err != nil {
		t.Fatalf("StartOnPort(0): %v", err)
	}
	if err := b.ServeWebUI(0); err != nil {
		t.Fatalf("ServeWebUI(0): %v", err)
	}
	brokerAddr := b.Addr()
	b.brokerRestartMu.Lock()
	webUIAddr := b.webUIListener.Addr().String()
	b.brokerRestartMu.Unlock()

	for _, addr := range []string{brokerAddr, webUIAddr} {
		if !acceptsConnections(t, addr) {
			t.Fatalf("listener %s not accepting before CloseListeners", addr)
		}
	}

	b.CloseListeners()
	// Idempotent: a second close (Stop also closes the broker listener)
	// must not panic.
	b.CloseListeners()

	// Give the kernel a beat to propagate the close, then require refusal.
	deadline := time.Now().Add(2 * time.Second)
	for _, addr := range []string{brokerAddr, webUIAddr} {
		for {
			if !acceptsConnections(t, addr) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("listener %s still accepting after CloseListeners", addr)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func acceptsConnections(t *testing.T, addr string) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// TestLauncherShutdownBoundedWhenDrainStalls is the regression test for the
// SIGTERM-immunity this change fixes: Broker.Stop waits on tracked background
// hooks with no deadline (bgWG.Wait), so a hook that never returns used to
// wedge Kill — and with it the whole signal path — forcing kill -9.
// Shutdown must (a) persist state BEFORE the drain, (b) return within its
// timeout even while the drain is wedged.
func TestLauncherShutdownBoundedWhenDrainStalls(t *testing.T) {
	// Isolate the runtime home so Kill's office PID/info sidecar cleanup
	// cannot touch the real ~/.hivex on the machine running the tests.
	t.Setenv("HIVEX_RUNTIME_HOME", t.TempDir())

	statePath := filepath.Join(t.TempDir(), "broker-state.json")
	b := NewBrokerAt(statePath)
	release := make(chan struct{})
	b.trackBackground(func() { <-release }) // wedge bgWG.Wait inside Stop

	l := &Launcher{broker: b}
	start := time.Now()
	l.Shutdown(300 * time.Millisecond)
	elapsed := time.Since(start)

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file absent after Shutdown — Persist did not run before the drain: %v", err)
	}
	if elapsed < 250*time.Millisecond {
		t.Fatalf("Shutdown returned after %s — did not wait for its budget (drain not actually wedged?)", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Shutdown returned after %s — drain wedge escaped the timeout", elapsed)
	}
	// Let the wedged drain goroutine finish so the test process doesn't
	// carry it past the assertion.
	close(release)
}

// TestLauncherShutdownWithoutBroker covers the nil-broker edge (Shutdown is
// called from the signal path before LaunchWeb has installed a broker): it
// must be a no-op, not a panic.
func TestLauncherShutdownWithoutBroker(t *testing.T) {
	t.Setenv("HIVEX_RUNTIME_HOME", t.TempDir())
	l := &Launcher{}
	done := make(chan struct{})
	go func() { l.Shutdown(50 * time.Millisecond); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown on broker-less Launcher did not return")
	}
}
