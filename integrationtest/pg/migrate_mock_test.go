package pg

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	corepg "github.com/ddd-qce/core/pg"
)

func TestMockMigrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	for i := 0; i < 22; i++ {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}

	err = corepg.Migrate(db)
	if err != nil {
		t.Errorf("Migrate() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockMigrate_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(".*").WillReturnError(errors.New("table exists"))

	err = corepg.Migrate(db)
	if err == nil {
		t.Error("Migrate() should return error on failure")
	}
}

func TestMockDropAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	for i := 0; i < 9; i++ {
		mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))
	}

	err = corepg.DropAll(db)
	if err != nil {
		t.Errorf("DropAll() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockDropAll_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(".*").WillReturnError(errors.New("connection lost"))

	err = corepg.DropAll(db)
	if err == nil {
		t.Error("DropAll() should return error on failure")
	}
}
