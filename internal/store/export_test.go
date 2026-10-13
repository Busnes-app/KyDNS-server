package store

import (
	"testing"

	"github.com/Busnes-app/ky-primitives/applysetup"
)

// SetBeforeTokenInsert and WaitCount expose test seams to the external store_test package.
func SetBeforeTokenInsert(f func()) { beforeTokenInsert = f }

func (s *Store) WaitCount() int64 { return s.db.Stats().WaitCount }

func InstallerBundleForTest(t *testing.T) applysetup.Common { return installerBundle(t) }
