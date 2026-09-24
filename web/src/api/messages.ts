/**
 * Channel messages and the slash-command registry.
 * Extracted from client.ts to keep that file inside the file-size budget
 * (scripts/check-file-size.sh, 1500-LOC ceiling). Re-exported by client.ts
 * so import sites are unchanged.
 */

import { trackOn } from "../lib/analytics";
import { get, post, requireChannel } from "./client";

export interface Message {
  id: string;
  from: string;
  channel: string;
  content: string;
  /**
   * Server-assigned message kind. Empty/absent for plain chat. Known kinds:
   *  - "agent_issue"        legacy bot-authored issue banner
   *  - "system_auth_error"  system-authored provider-auth failure card (#933)
   *  - "ceo_*"              onboarding cards (form_field, chip_row, etc.)
   * The SPA's MessageBubble dispatches on this field to pick a renderer.
   */
  kind?: string;
  /**
   * Structured card payload for kinds that carry one. The broker marshals
   * this from a Go json.RawMessage so consumers receive an inline JSON
   * object (or array) — not a string. Consumers must treat every string
   * field inside as plain text (defense in depth on top of the broker-side
   * sanitizeContextValue).
   */
  payload?: unknown;
  redacted?: boolean;
  redaction_count?: number;
  redaction_reasons?: string[];
  timestamp: string;
  reply_to?: string;
  thread_id?: string;
  thread_count?: number;
  /**
   * Two shapes reach the client and both are real.
   *
   * The map form is `{ "👀": ["cos", "eng"] }` — emoji to the slugs that
   * reacted. The array form is a pre-counted `[{ emoji, count }]`. MessageBubble
   * has always handled both, branching on Array.isArray and casting, because
   * the cast was the only way past a type that claimed only one of them existed.
   *
   * The type was the wrong half of that disagreement, not the code. A cast that
   * exists to work around a declaration is a note saying the declaration is
   * lying; widening it removes the cast and lets a test construct either shape
   * without pretending.
   */
  reactions?:
    | Record<string, string[]>
    | Array<{ emoji: string; count?: number; reacted?: boolean }>;
  tagged?: string[];
  usage?: TokenUsage;
}

export interface TokenUsage {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_tokens?: number;
  cache_creation_tokens?: number;
  total_tokens?: number;
  cost_usd?: number;
}

/** Coarse length bucket for a message body — never the content itself. */
function lengthBucket(text: string): "empty" | "short" | "medium" | "long" {
  const n = text.trim().length;
  if (n === 0) return "empty";
  if (n < 80) return "short";
  if (n < 400) return "medium";
  return "long";
}

export function getMessages(
  channel: string,
  sinceId?: string | null,
  limit = 50,
) {
  return get<{ messages: Message[] }>("/messages", {
    channel: requireChannel(channel, "getMessages"),
    viewer_slug: "human",
    since_id: sinceId ?? null,
    limit,
  });
}

export function postMessage(
  content: string,
  channel: string,
  replyTo?: string,
  tagged?: string[],
) {
  const body: Record<string, string | string[]> = {
    from: "you",
    channel: requireChannel(channel, "postMessage"),
    content,
  };
  if (replyTo) body.reply_to = replyTo;
  if (tagged && tagged.length > 0) body.tagged = tagged;
  return trackOn(post<Message>("/messages", body), "message_sent", {
    is_reply: !!replyTo,
    mention_count: tagged?.length ?? 0,
    length_bucket: lengthBucket(content),
  });
}

export function getThreadMessages(channel: string, threadId: string) {
  return get<{ messages: Message[] }>("/messages", {
    channel: requireChannel(channel, "getThreadMessages"),
    thread_id: threadId,
    viewer_slug: "human",
    limit: 50,
  });
}

export function toggleReaction(msgId: string, emoji: string, channel: string) {
  return post("/messages/react", {
    message_id: msgId,
    emoji,
    channel: requireChannel(channel, "toggleReaction"),
  });
}

// ── Slash-command registry ──

/**
 * One entry from GET /commands. Mirrors the broker's `commandDescriptor`
 * shape in internal/team/broker_commands.go. Sorted alphabetically by the
 * broker — callers do not need to re-sort.
 */
export interface SlashCommandDescriptor {
  name: string;
  description: string;
  /** True when the web composer has a real handler for the command. */
  webSupported: boolean;
}

/**
 * Fetch the canonical slash-command registry from the broker. The web
 * autocomplete filters to webSupported=true; other callers may want the
 * full set for discovery.
 */
export function fetchCommands() {
  return get<SlashCommandDescriptor[]>("/commands");
}
