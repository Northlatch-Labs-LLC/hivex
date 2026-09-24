package gridframe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// persist.go — write-through persistence for the ledger store. When armed
// with PersistTo, every successful mutation (append, reversal, import,
// month-lock) is flushed to the ledger directory as byte-exact CSV (§5.1),
// so a broker restart reloads the books unchanged. Files are written
// atomically (temp + rename) so a crash mid-write never truncates a ledger.

const locksFile = ".locks.json"

// PersistTo arms write-through persistence to dir and loads any tables and
// month-locks already persisted there (over the in-memory content).
func (s *Store) PersistTo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := s.loadFrom(dir); err != nil {
		return err
	}
	s.persistDir = dir
	return nil
}

// SeedFrom copies each seed CSV from seedDir into the persistence directory
// for tables that have no file there yet (first boot). It is a no-op when
// the ledgers already exist.
func (s *Store) SeedFrom(seedDir string) error {
	if s.persistDir == "" {
		return fmt.Errorf("gridframe: PersistTo before SeedFrom")
	}
	// Only tables that had NO file get seeded; books already on disk were
	// loaded by PersistTo and must not be re-imported — append-only rejects
	// duplicates, which used to turn every re-boot into a spurious
	// "seed ledgers unavailable" error.
	seeded := false
	for _, t := range Tables() {
		dst := filepath.Join(s.persistDir, t.Name+".csv")
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(seedDir, t.Name+".csv"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := atomicWrite(dst, raw); err != nil {
			return err
		}
		seeded = true
	}
	if !seeded {
		return nil
	}
	return s.loadFrom(s.persistDir)
}

func (s *Store) loadFrom(dir string) error {
	for _, t := range Tables() {
		raw, err := os.ReadFile(filepath.Join(dir, t.Name+".csv"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := s.ImportTable(t.Name, raw); err != nil {
			return fmt.Errorf("gridframe: load %s: %w", t.Name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, locksFile))
	if err == nil {
		var locks []string
		if json.Unmarshal(raw, &locks) == nil {
			for _, m := range locks {
				s.LockMonth(m)
			}
		}
	}
	return nil
}

// persistTable flushes one table's CSV. Called after in-memory mutations.
func (s *Store) persistTable(name string) {
	if s.persistDir == "" {
		return
	}
	td, err := s.Table(name)
	if err != nil {
		return
	}
	_ = atomicWrite(filepath.Join(s.persistDir, name+".csv"), td.Export())
}

// persistLocks flushes the month-lock set.
func (s *Store) persistLocks() {
	if s.persistDir == "" {
		return
	}
	locks := s.LockedMonths()
	raw, err := json.Marshal(locks)
	if err != nil {
		return
	}
	_ = atomicWrite(filepath.Join(s.persistDir, locksFile), raw)
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
