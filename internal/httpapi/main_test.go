package httpapi

import (
	"testing"

	"github.com/sami0076/tennis-wiki/internal/testdb"
)

// The test database is shared across this package's tests, so its cleanup
// belongs here rather than to any one test.
func TestMain(m *testing.M) { testdb.Run(m) }
