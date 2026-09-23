package team

import (
	"encoding/json"
	"net/http"

	"github.com/Northlatch-Labs-LLC/hivex/internal/marketplace"
)

// handleMarketplaceCatalog is GET /marketplace — the curated catalog with
// each entry's installed state merged in, plus nothing else writable.
func (b *Broker) handleMarketplaceCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	installed, err := marketplace.Installed()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type entryWithState struct {
		marketplace.Entry
		Installed bool `json:"installed"`
	}
	out := make([]entryWithState, 0, len(marketplace.Catalog()))
	for _, e := range marketplace.Catalog() {
		out = append(out, entryWithState{Entry: e, Installed: installed[e.ID]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// handleMarketplaceInstall is POST /marketplace/install {id}.
func (b *Broker) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request) {
	b.marketplaceByID(w, r, func(id string) error {
		entry, _ := marketplace.FindEntry(id)
		return marketplace.Install(entry)
	})
}

// handleMarketplaceUninstall is POST /marketplace/uninstall {id}.
func (b *Broker) handleMarketplaceUninstall(w http.ResponseWriter, r *http.Request) {
	b.marketplaceByID(w, r, marketplace.Uninstall)
}

func (b *Broker) marketplaceByID(w http.ResponseWriter, r *http.Request, act func(string) error) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !marketplace.ValidateEntryID(body.ID) {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := act(body.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b.handleMarketplaceCatalog(w, r)
}
