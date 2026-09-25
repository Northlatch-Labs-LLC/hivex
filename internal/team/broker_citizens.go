package team

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
	"github.com/Northlatch-Labs-LLC/hivex/internal/gridframe"
	"github.com/Northlatch-Labs-LLC/hivex/internal/provider"
)

// The citizen provisioning primitive — the unit Gridframe sells: one
// agent-citizen = one machine + one inference key, metered into the ledgers.
//
// POST /citizens {weir_handle} provisions an office member through the exact
// office-members create path (createOfficeMember in
// broker_office_members.go) with provider kind "zai" and the default GLM
// model, mints a synthetic inference key sk-hivex-<24 hex> (crypto/rand),
// persists it in the config custom section (plain, like zai_api_key), and
// appends a cost-ledger row so the books carry the provisioning event.
//
// The plaintext key crosses the wire exactly once, in the creation response.
// Every later surface shows config.KeyFingerprint only; DELETE /citizens/{slug}
// revokes the member and destroys the stored key.

// citizenKeyHexChars is the random-hex tail of a synthetic key: 24 hex chars
// (12 bytes of entropy) behind the fixed "sk-hivex-" prefix.
const citizenKeyHexChars = 24

func (b *Broker) handleCitizensPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WeirHandle string `json:"weir_handle"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}
	handle := strings.TrimSpace(body.WeirHandle)
	// Raw-emptiness refusal BEFORE normalizeChannelSlug: the normaliser
	// launders "" into the #general slug, so a missing handle must be
	// rejected here (same boundary-rule as serveOfficeMemberMutation).
	if handle == "" {
		http.Error(w, "weir_handle required", http.StatusBadRequest)
		return
	}
	slug := normalizeChannelSlug(handle)

	key, err := generateCitizenKey()
	if err != nil {
		http.Error(w, "failed to mint inference key", http.StatusInternalServerError)
		return
	}

	// Provision through the office-members create path semantics: same
	// parser/applier split, same conflict checks, same channel-roster
	// seeding and persistence the /office-members action=create route uses.
	b.officeMemberMutationMu.Lock()
	result, mutationErr := b.createOfficeMember(r, slug, officeMemberMutationBody{
		Slug:      slug,
		CreatedBy: "citizens-api",
		Provider: &provider.ProviderBinding{
			Kind:  provider.KindZAI,
			Model: provider.ZaiDefaultModel(),
		},
	})
	b.officeMemberMutationMu.Unlock()
	if mutationErr != nil {
		http.Error(w, mutationErr.message, mutationErr.status)
		return
	}
	if err := b.writeBrokerState(result.write); err != nil {
		http.Error(w, "failed to persist broker state", http.StatusInternalServerError)
		return
	}
	b.publishOfficeChanges(result.events)
	if result.ensureNotebookDirs {
		b.backfillBotFilesForRoster()
	}

	provisionedAt := time.Now().UTC().Format(time.RFC3339)

	// Persist the key before responding: once the response is written the
	// caller has seen the only plaintext copy that will ever exist, so the
	// durable record must already be in place.
	if err := saveCitizenKey(slug, key, provisionedAt); err != nil {
		log.Printf("citizens: provision %q: key persist failed: %v", slug, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "member provisioned but key persistence failed: " + err.Error(),
			"slug":  slug,
			"hint":  "revoke via DELETE /citizens/" + slug + " and retry",
		})
		return
	}

	// Meter the provisioning into the cost ledger. A failure here is NOT
	// silenced and does NOT return the key: the citizen exists and its key
	// is stored, but the response reports the failure so the operator can
	// revoke and retry rather than trust an unbooked citizen.
	ledger := b.citizenLedgerStore()
	if ledger == nil {
		log.Printf("citizens: provision %q: gridframe ledger store unavailable", slug)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "member provisioned and key stored, but the gridframe ledger store is unavailable; revoke and retry",
			"slug":  slug,
		})
		return
	}
	if err := appendCitizenProvisionLedgerRow(ledger, slug, time.Now()); err != nil {
		log.Printf("citizens: provision %q: ledger append failed: %v", slug, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "member provisioned and key stored, but the cost-ledger append failed: " + err.Error(),
			"slug":  slug,
			"hint":  "revoke via DELETE /citizens/" + slug + " and retry",
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"slug":           slug,
		"key":            key,
		"provisioned_at": provisionedAt,
	})
}

func (b *Broker) handleCitizensDelete(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("slug")
	// Raw-emptiness refusal before normalisation, same rule as POST.
	if strings.TrimSpace(raw) == "" {
		http.Error(w, "slug required", http.StatusBadRequest)
		return
	}
	slug := normalizeChannelSlug(raw)

	b.officeMemberMutationMu.Lock()
	result, mutationErr := b.removeOfficeMember(r, slug)
	b.officeMemberMutationMu.Unlock()
	if mutationErr != nil {
		http.Error(w, mutationErr.message, mutationErr.status)
		return
	}
	if err := b.writeBrokerState(result.write); err != nil {
		http.Error(w, "failed to persist broker state", http.StatusInternalServerError)
		return
	}
	b.publishOfficeChanges(result.events)

	// The office-members model has no separate archived state — "remove" is
	// the revocation semantics (roster, channels, tasks all reconcile), so
	// the key must go with it or the citizen could still authenticate.
	if err := removeCitizenKey(slug); err != nil {
		log.Printf("citizens: revoke %q: key removal failed: %v", slug, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "member revoked but key removal failed: " + err.Error(),
			"slug":  slug,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"slug":       slug,
		"revoked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleCitizens is POST /citizens: provision one agent-citizen.
func (b *Broker) handleCitizens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b.handleCitizensPost(w, r)
}

// handleCitizensSubpath is DELETE /citizens/{slug}: revoke one agent-citizen.
func (b *Broker) handleCitizensSubpath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b.handleCitizensDelete(w, r)
}

// citizenLedgerStore returns the live gridframe store the /gridframe HTTP
// surface serves, or nil when the broker was never launched (no StartOnPort,
// so no ledger home was armed). Guarded by gridframeMu: the gridframe Store
// is not internally synchronized, so this package's appends serialize on it.
func (b *Broker) citizenLedgerStore() *gridframe.Store {
	b.gridframeMu.Lock()
	defer b.gridframeMu.Unlock()
	return b.gridframeStore
}

// ensureGridframeStore lazily constructs the shared ledger store exactly once
// (StartOnPort then arms persistence + seeding on the returned instance).
func (b *Broker) ensureGridframeStore() *gridframe.Store {
	b.gridframeMu.Lock()
	defer b.gridframeMu.Unlock()
	if b.gridframeStore == nil {
		b.gridframeStore = gridframe.NewStore()
	}
	return b.gridframeStore
}

// generateCitizenKey mints sk-hivex-<24 lowercase hex chars> from crypto/rand.
// The prefix keeps synthetic keys visually distinct from vendor credentials;
// the tail is 96 bits of entropy, unguessable and collision-free in practice.
func generateCitizenKey() (string, error) {
	buf := make([]byte, citizenKeyHexChars/2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("citizens: crypto/rand: %w", err)
	}
	return "sk-hivex-" + hex.EncodeToString(buf), nil
}

// saveCitizenKey stores the provisioned key in the config custom section
// under customProvidersMu (the same Load→mutate→Save serialization the zai
// key write path uses — see broker_zai_key.go). Keyed by slug: a retry after
// a failed save replaces the stale record instead of duplicating it.
func saveCitizenKey(slug, key, provisionedAt string) error {
	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	next := make([]config.CitizenKey, 0, len(cfg.CitizenKeys)+1)
	for _, ck := range cfg.CitizenKeys {
		if ck.Slug != slug {
			next = append(next, ck)
		}
	}
	next = append(next, config.CitizenKey{Slug: slug, APIKey: key, CreatedAt: provisionedAt})
	cfg.CitizenKeys = next
	return config.Save(cfg)
}

// removeCitizenKey destroys the stored key for slug. An absent record is a
// no-op success: revoke is idempotent and a provision that failed before its
// key was persisted must still be revocable.
func removeCitizenKey(slug string) error {
	customProvidersMu.Lock()
	defer customProvidersMu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	next := make([]config.CitizenKey, 0, len(cfg.CitizenKeys))
	for _, ck := range cfg.CitizenKeys {
		if ck.Slug != slug {
			next = append(next, ck)
		}
	}
	if len(next) == len(cfg.CitizenKeys) {
		return nil
	}
	cfg.CitizenKeys = next
	return config.Save(cfg)
}

// citizenLedgerTxnID mints a never-used CST-<date>-CZ<hex> txn_id. The seed
// convention is CST-YYYYMMDD-NN; the CZ-prefixed random tail keeps provision
// rows in the same shape while dodging the sequence-number race between
// concurrent provisions (Store.Append still rejects any collision, so we
// re-roll; a persistent collision after 8 attempts is a real error).
func citizenLedgerTxnID(td *gridframe.TableData, sch gridframe.Table, date string) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		buf := make([]byte, 3)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("citizens: crypto/rand: %w", err)
		}
		id := "CST-" + strings.ReplaceAll(date, "-", "") + "-CZ" + hex.EncodeToString(buf)
		used := false
		for _, row := range td.Rows {
			if row.Get(sch, sch.IDCol) == id {
				used = true
				break
			}
		}
		if !used {
			return id, nil
		}
	}
	return "", fmt.Errorf("citizens: no free txn_id after 8 attempts")
}

// appendCitizenProvisionLedgerRow books one citizen provisioning into the
// Gridframe cost ledger via the Store.Append API. The cost-ledger schema
// (schema.go §5.1) has no owner column — the seeds carry attribution through
// block/category/vendor (e.g. "A,agent-inference,…,zai-monthly",
// "D,compute,Customer environment compute pool"), so a provisioned citizen —
// a customer-environment machine + inference key — books as block D,
// category compute, vendor gridframe, description citizen-provision:<slug>.
// Amount is 0.00: provisioning itself moves no cash; the citizen's inference
// meters separately into b.usage.ByKind via its member binding.
func appendCitizenProvisionLedgerRow(s *gridframe.Store, slug string, provisionedAt time.Time) error {
	sch, ok := gridframe.Lookup(gridframe.TCost)
	if !ok {
		return fmt.Errorf("gridframe: unknown table %q", gridframe.TCost)
	}
	td, err := s.Table(gridframe.TCost)
	if err != nil {
		return err
	}
	date := provisionedAt.UTC().Format("2006-01-02")
	txnID, err := citizenLedgerTxnID(td, sch, date)
	if err != nil {
		return err
	}
	row := gridframe.Row{
		txnID,
		date,
		"D",
		"compute",
		"citizen-provision:" + slug,
		"0.00",
		"gridframe",
		"",
	}
	if err := s.Append(gridframe.TCost, row); err != nil {
		return fmt.Errorf("cost-ledger append: %w", err)
	}
	return nil
}
