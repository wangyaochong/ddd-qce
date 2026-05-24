package infra_pg

import (
	"testing"

	"github.com/ddd-qce/core/infra/infratest"
	corepgx "github.com/ddd-qce/core/pgx"
	"github.com/ddd-qce/it/testutil"
)

func TestPGBackend_Contract(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_backend_test")
	b := corepgx.NewPGBackend(db)
	infratest.TestBackendContract(t, b)
}
