// Package marketplace: the curated catalog of installable skills, bot
// experts, and plugins for the office Marketplace, plus the install
// service that materializes entries into the workspace wiki.
package marketplace

import "strings"

// Category of a catalog entry. Skills and experts install as markdown
// files; plugins install as JSON manifests.
type Category string

const (
	CategorySkill  Category = "skill"
	CategoryExpert Category = "expert"
	CategoryPlugin Category = "plugin"
)

// Entry is one catalog item. Payload is the file body written on install.
type Entry struct {
	ID          string   `json:"id"`
	Category    Category `json:"category"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Payload     string   `json:"payload"`
}

// Catalog returns the curated seed set shipped with the harness.
func Catalog() []Entry {
	return []Entry{
		{
			ID:          "code-review",
			Category:    CategorySkill,
			Name:        "Code review",
			Description: "Review a diff for correctness, security, and test coverage before merge.",
			Payload: `---
name: code-review
description: Review a diff for correctness, security, and test coverage before merge.
version: 1.0.0
metadata:
  hivex:
    title: Code review
    created_by: hivex
    status: active
---
Review the given diff. Report findings ordered by severity with file:line
references, then list what was checked and found clean.`,
		},
		{
			ID:          "release-notes",
			Category:    CategorySkill,
			Name:        "Release notes",
			Description: "Draft user-facing release notes from a changelog or commit range.",
			Payload: `---
name: release-notes
description: Draft user-facing release notes from a changelog or commit range.
version: 1.0.0
metadata:
  hivex:
    title: Release notes
    created_by: hivex
    status: active
---
Draft plain-language release notes from the given commits. Group by Added,
Changed, Fixed. No invented entries.`,
		},
		{
			ID:          "sre-on-call",
			Category:    CategoryExpert,
			Name:        "SRE on-call",
			Description: "Emergency-operations expert: triage incidents, stabilize, write postmortems.",
			Payload: `---
name: sre-on-call
description: Emergency-operations expert: triage incidents, stabilize, write postmortems.
version: 1.0.0
metadata:
  hivex:
    title: SRE on-call
    created_by: hivex
    status: active
    role: expert
---
Act as the on-call SRE. Triage by blast radius, stabilize first, document
as you go, hand back a timeline postmortem with owners.`,
		},
		{
			ID:          "data-analyst",
			Category:    CategoryExpert,
			Name:        "Data analyst",
			Description: "Turns raw ledgers and logs into concise findings with the math shown.",
			Payload: `---
name: data-analyst
description: Turns raw ledgers and logs into concise findings with the math shown.
version: 1.0.0
metadata:
  hivex:
    title: Data analyst
    created_by: hivex
    status: active
    role: expert
---
Analyze the given data. State assumptions, show the arithmetic, and rank
findings by materiality. Never present a number you did not compute.`,
		},
	}
}

// FindEntry returns the catalog entry with the given ID.
func FindEntry(id string) (Entry, bool) {
	id = strings.TrimSpace(id)
	for _, e := range Catalog() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}
