package team

// Graceful-shutdown primitives for the SIGTERM/SIGINT path (deploy behind a
// supervisor: docker compose, systemd). The full Broker.Stop teardown waits
// on subprocess-draining WaitGroups with no overall deadline, which is why a
// signalled engine historically looked like it ignored SIGTERM and had to be
// kill -9'd. These two methods are the bounded half of the contract:
//
//	Persist        — flush broker state to broker-state.json (atomic rename)
//	CloseListeners — release the broker API and web UI TCP listeners
//
// Launcher.Shutdown runs both first, then attempts the full Kill/Stop drain
// under the remaining budget, so "state saved, listeners closed" holds even
// when a stuck subprocess would otherwise stall the drain.

// Persist flushes the in-memory broker state to disk via saveLocked — the
// same write-through path every mutation uses — without tearing anything
// down. Safe to call at any point in the broker's life, including after Stop
// has begun (the state file write is idempotent). Returns the save error so
// the shutdown path can log it; shutdown proceeds either way.
func (b *Broker) Persist() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.saveLocked()
}

// CloseListeners releases both HTTP listeners (broker API + web UI) and
// closes their servers. Idempotent: net.Listener.Close and http.Server.Close
// on an already-closed listener/server return an error that is deliberately
// ignored. This is the "close the listeners" half of the shutdown contract;
// Stop also closes the broker API listener (broker.go Stop), so a full Stop
// after CloseListeners simply no-ops there. Force-closing (rather than
// graceful http.Server.Shutdown) is intentional: in-flight SSE streams
// (bot-stream, events) are open-ended and would otherwise hold the listener
// for their entire lifetime.
func (b *Broker) CloseListeners() {
	if b == nil {
		return
	}
	b.brokerRestartMu.Lock()
	defer b.brokerRestartMu.Unlock()
	if b.webUIListener != nil {
		_ = b.webUIListener.Close()
	}
	if b.webUIServer != nil {
		_ = b.webUIServer.Close()
	}
	if b.listener != nil {
		_ = b.listener.Close()
	}
	if b.server != nil {
		_ = b.server.Close()
	}
}
