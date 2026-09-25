/**
 * LatestTurnLine — the agent subspace's compact live-work line: what the
 * turn engine says this agent's current (or most recent) turn is doing,
 * for how long, and at what token cost. One small line in the header, not
 * a tab — it answers "is something happening right now?", which is the
 * question the header's status badge already answers for the adapter;
 * this is the engine's answer. Renders nothing until the journal has a
 * turn for this agent.
 */

import { useEffect, useState } from "react";

import { isTurnInFlight, useTurnsLive } from "../../hooks/useTurnsLive";
import { formatTokens } from "../../lib/format";

/** "context_assembly" → "context assembly" — the engine's stages are
 * snake_case; a human reads them with spaces. */
function humanizeState(state: string): string {
  return state.replaceAll("_", " ");
}

/** Compact wall duration for an in-flight turn: "45s", "1m 30s", "1h 04m".
 * Deliberately not formatRelative — that appends "ago", and a running
 * elapsed is not in the past. */
function formatElapsed(totalSeconds: number): string {
  const seconds = Math.max(0, totalSeconds);
  const minutes = Math.floor(seconds / 60);
  if (minutes === 0) return `${seconds}s`;
  if (minutes < 60) {
    const rem = seconds % 60;
    return rem === 0 ? `${minutes}m` : `${minutes}m ${rem}s`;
  }
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${String(minutes % 60).padStart(2, "0")}m`;
}

function turnTokenTotal(usage: {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_tokens?: number;
  cache_creation_tokens?: number;
}): number {
  return (
    (usage.input_tokens ?? 0) +
    (usage.output_tokens ?? 0) +
    (usage.cache_read_tokens ?? 0) +
    (usage.cache_creation_tokens ?? 0)
  );
}

export function LatestTurnLine({ slug }: { slug: string }) {
  // Same query key as the sidebar's working dot, so the rail and this
  // line share one poll and can never disagree about "is it live".
  const turn = useTurnsLive().data?.[slug];
  const inFlight = isTurnInFlight(turn);
  const [nowMs, setNowMs] = useState(() => Date.now());

  // The clock only runs while the turn does: a terminal turn's elapsed is
  // frozen at updated_at, so no interval is kept alive to re-render it.
  useEffect(() => {
    if (!inFlight) return;
    const id = globalThis.setInterval(() => setNowMs(Date.now()), 1000);
    return () => globalThis.clearInterval(id);
  }, [inFlight]);

  if (!turn) return null;

  const startMs = Date.parse(turn.started_at);
  const endMs = inFlight ? nowMs : Date.parse(turn.updated_at);
  const elapsed =
    Number.isFinite(startMs) && Number.isFinite(endMs) && endMs >= startMs
      ? formatElapsed(Math.floor((endMs - startMs) / 1000))
      : null;
  const tokens = turn.usage ? turnTokenTotal(turn.usage) : null;

  return (
    <div
      className="bot-subspace-latest-turn"
      data-testid="latest-turn-line"
      title={`Latest turn: ${humanizeState(turn.state)}${
        elapsed ? ` · ${elapsed}` : ""
      }${turn.model ? ` · ${turn.model}` : ""}`}
    >
      <span>Latest turn</span>
      <span
        className={
          inFlight ? "bot-subspace-latest-turn__state--live" : undefined
        }
        data-testid="latest-turn-state"
      >
        {humanizeState(turn.state)}
      </span>
      {elapsed ? <span>{elapsed}</span> : null}
      {tokens ? <span>{formatTokens(tokens)} tok</span> : null}
    </div>
  );
}
