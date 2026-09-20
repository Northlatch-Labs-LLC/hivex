"""
MEMANTO CLI - Theme Constants

Centralized color palette inspired by Moorcheh's blue-violet branding.
All CLI output should use these constants rather than hard-coded color names.
"""

# ── Primary palette ────────────────────────────────────────────────
# Main brand color (blue-violet) — headers, borders, accents
PRIMARY = "medium_purple3"
# Brighter variant — highlighted commands, emphasized items
BRIGHT = "slate_blue1"
# Softer variant — secondary column highlights, type badges
ACCENT = "plum3"

# ── Semantic colors (used only for status indicators) ──────────────
SUCCESS = "green"
ERROR = "red"
WARNING = "yellow"
DIM = "dim"

# ── Composite Rich markup strings ──────────────────────────────────
BOLD_PRIMARY = f"bold {PRIMARY}"
BOLD_BRIGHT = f"bold {BRIGHT}"
