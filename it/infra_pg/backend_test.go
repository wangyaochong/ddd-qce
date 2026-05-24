package infra_pg

import (
	"testing"

	"github.com/ddd-qce/core/infra"
	"github.com/ddd-qce/core/infra/infratest"
	"github.com/ddd-qce/it/testutil"
)

func TestPGBackend_Contract(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_backend_test")
	b := infra.NewPgBackend(db)
	infratest.TestBackendContract(t, b)
}
