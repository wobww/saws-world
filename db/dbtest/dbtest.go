package dbtest

import (
	"os"

	"github.com/pkg/errors"
	"github.com/wobwainwwight/sa-photos/db"
)

type TestImageTable struct {
	path string
	db.ImageTable
}

func NewTestImageTable() *TestImageTable {
	path := "image_table_test.db"
	t, err := db.NewImageTable(path)
	if err != nil {
		panic(errors.Wrap(err, "could not setup test image table"))
	}

	return &TestImageTable{
		path:       path,
		ImageTable: t,
	}
}

func (t *TestImageTable) Close() error {
	err := os.Remove(t.path)
	if err != nil {
		return errors.Wrap(err, "error while closing test image table")
	}
	return nil
}
