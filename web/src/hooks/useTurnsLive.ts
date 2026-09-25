import { useQuery } from "@tanstack/react-query";

import type { LiveTurn } from "../api/client";
import { getTurnsLive } from "../api/client";

/**
 * The turn engine's terminal stages. A turn in ANY other state (intake,
 * dispatch, executing, verifying, …) means the agent is on a turn right
 * now — that is the "dispatch" umbrella the sidebar's working dot lights
 * on. An unknown state is treated as not live: the dot must never claim
 * work the journal doesn't vouch for.
 */
const TERMINAL_TURN_STATES = new Set(["settled", "failed"]);

export function isTurnInFlight(turn: LiveTurn | undefined): boolean {
  if (!turn) return false;
  return !TERMINAL_TURN_STATES.has(turn.state);
}

/**
 * Latest turn per agent, polled from GET /turns/live. Polls faster than
 * the member list (2s vs 5s) because a mid-turn dot is only useful while
 * it is roughly true; degrades to an empty map whenever the broker is
 * unreachable — callers treat "no data" as "not live", never as an error
 * surface.
 */
export function useTurnsLive() {
  return useQuery({
    queryKey: ["turns-live"],
    queryFn: () => getTurnsLive(),
    refetchInterval: 2000,
    select: (data): Record<string, LiveTurn> => {
      const byAgent: Record<string, LiveTurn> = {};
      for (const turn of data.turns ?? []) {
        if (turn?.agent) byAgent[turn.agent] = turn;
      }
      return byAgent;
    },
  });
}
