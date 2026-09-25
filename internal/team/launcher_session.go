package team

// launcher_session.go owns the user-facing session state-change
// methods (PLAN.md §C20): Attach (re-attach the user's terminal to
// the running tmux session), Kill (graceful drain + tear-down), and
// ResetSession (clear broker state + manifest + per-bot temp files
// for a fresh-start). Each is small (15-30 lines) but together they
// form the "user can drive the running team" surface — separate from
// boot (Launch) and reconfigure (ReconfigureSession).

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// Attach attaches the user's terminal to the tmux session.
// In iTerm2: uses tmux -CC for native panes (resizable, close buttons, drag).
// Otherwise: uses regular tmux attach with -L hivex to avoid nesting.
func (l *Launcher) Attach() error {
	var cmd *exec.Cmd
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		// tmux -CC mode: iTerm2 takes over window management.
		// Creates native iTerm2 tabs/splits for each tmux window/pane.
		cmd = exec.CommandContext(context.Background(), "tmux", "-L", tmuxSocketName, "-CC", "attach-session", "-t", l.sessionName)
	} else {
		cmd = exec.CommandContext(context.Background(), "tmux", "-L", tmuxSocketName, "attach-session", "-t", l.sessionName)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Unset TMUX env to allow nesting
	cmd.Env = filterEnv(os.Environ(), "TMUX")
	return cmd.Run()
}

// Kill destroys the tmux session, all bot processes, and the broker. Also
// removes per-bot temp files (MCP config + system prompt) so the broker
// token and prompt content do not linger in $TMPDIR.
//
// Drains every long-lived goroutine before broker.Stop and the
// per-launch tempdir RemoveAll. Pre-fix the scheduler goroutine
// outlived Kill (now drained via schedulerWorker.Stop) but the
// headless workers were only context-cancelled — cancel kicks the
// subprocess but the worker goroutine takes a tick to unwind, and
// it can race os.RemoveAll(launchTempDirPath) inside
// cleanupBotTempFiles by writing a fresh per-bot prompt/MCP
// file into a directory the cleanup is removing.
//
// Sequence:
//  1. schedulerWorker.Stop()    — drain watchdog
//  2. headless.cancel()         — kick subprocess
//  3. stopHeadlessWorkers()     — wait on workerWg
//  4. broker.Stop()             — listener teardown
//  5. cleanupBotTempFiles()   — rm -rf launch dir
func (l *Launcher) Kill() error {
	if l.schedulerWorker != nil {
		l.schedulerWorker.Stop()
	}
	if l.headless.cancel != nil {
		l.headless.cancel()
	}
	l.stopHeadlessWorkers()
	if l.broker != nil {
		l.broker.Stop()
	}
	// Clean temp files before tearing down tmux so the claude processes are
	// still alive to release any open handles (harmless, but principle of
	// least surprise).
	l.cleanupBotTempFiles()
	// Clear the office.json attach sidecar on every shutdown path (both the
	// headless-web and pane runtimes) so a clean exit never leaves a front-end
	// pointing at a dead URL. (office.pid clearing stays runtime-specific below.)
	_ = clearOfficeInfo()
	if !l.targeter().UsesPaneRuntime() {
		if err := killPersistedOfficeProcess(); err != nil {
			return err
		}
		killStaleHeadlessTaskRunners()
		_ = clearOfficePIDFile()
		return nil
	}
	out, err := exec.CommandContext(context.Background(), "tmux", "-L", tmuxSocketName, "kill-session", "-t", l.sessionName).CombinedOutput()
	if err != nil {
		// "session not found" is the desired post-condition for
		// Kill, not an error — caller's intent is "make sure the
		// session is gone". isMissingTmuxSession matches tmux's
		// "can't find" / "no server" / "error connecting" outputs,
		// so kill-twice-in-a-row no longer surfaces a misleading
		// exit-1 to the caller.
		if isMissingTmuxSession(string(out)) {
			return nil
		}
		return err
	}
	return nil
}

// Shutdown stops a running office with a hard deadline. It is the SIGTERM/
// SIGINT path for supervised deployments (docker compose, systemd), where
// Kill alone could stall past any supervisor's patience: Kill waits on
// subprocess-draining WaitGroups (stopHeadlessWorkers, Broker.Stop's
// bgWG.Wait) with no overall bound, so a stuck bot subprocess made a
// signalled engine look SIGTERM-immune and earned it a kill -9.
//
// Order matters and mirrors the deployment contract — save state, close
// listeners, exit within the budget:
//  1. Broker.Persist — final write-through of broker-state.json (fast,
//     atomic rename; the same path every mutation already uses).
//  2. Broker.CloseListeners — release the broker API + web UI listeners so
//     the ports stop accepting the moment we decide to go.
//  3. Kill with the remaining budget — full drain (watchdog, headless
//     workers, broker Stop, temp files, office PID/info sidecars). If the
//     drain exceeds the timeout we return anyway: state is saved and the
//     listeners are closed, which is the guaranteed half of the contract.
//     The caller (cmd/hivex runWeb) exits 0 immediately after.
func (l *Launcher) Shutdown(timeout time.Duration) {
	if l == nil {
		return
	}
	if l.broker != nil {
		if err := l.broker.Persist(); err != nil {
			log.Printf("launcher shutdown: final broker state persist failed: %v", err)
		}
		l.broker.CloseListeners()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := l.Kill(); err != nil {
			log.Printf("launcher shutdown: kill: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Printf("launcher shutdown: drain did not finish within %s; state is persisted and listeners are closed — exiting anyway", timeout)
	}
}

func (l *Launcher) ResetSession() error {
	if !l.targeter().RequiresClaudeSessionReset() {
		if l != nil && l.broker != nil {
			l.broker.Reset()
			return nil
		}
		if err := ResetBrokerState(); err != nil {
			return fmt.Errorf("reset broker state: %w", err)
		}
		return nil
	}
	if err := provider.ResetClaudeSessions(); err != nil {
		return fmt.Errorf("reset Claude sessions: %w", err)
	}
	if l != nil && l.broker != nil {
		l.broker.Reset()
		return nil
	}
	if err := ResetBrokerState(); err != nil {
		return fmt.Errorf("reset broker state: %w", err)
	}
	return nil
}
