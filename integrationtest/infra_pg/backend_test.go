package infra_pg

import (
	"testing"

	"github.com/ddd-qce/core/infra"
	"github.com/ddd-qce/core/infra/infratest"
	"github.com/ddd-qce/integrationtest/testutil"
)

func TestPGBackend_Contract(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.CleanDB(t, db)
	b := infra.NewPgBackend(db)
	infratest.TestBackendContract(t, b)
}
