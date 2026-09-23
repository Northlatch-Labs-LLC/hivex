package gridframe

import (
	"os"
	"path/filepath"
)

// SeedStore imports the 8 §5.2 seed CSVs from dir into the store, table by
// table, through ImportTable (append-only semantics enforced).
func (s *Store) SeedStore(dir string) error {
	for _, name := range TableNames() {
		raw, err := os.ReadFile(filepath.Join(dir, name+".csv"))
		if err != nil {
			return err
		}
		if err := s.ImportTable(name, raw); err != nil {
			return err
		}
	}
	return nil
}
