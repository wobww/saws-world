package db_test

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
)

func TestDB(t *testing.T) {

	t.Run("should setup saws.db if no arg", func(t *testing.T) {
		require.NoFileExists(t, "saws.db")
		_, err := db.NewImageTable("")
		require.NoError(t, err)

		stat, err := os.Stat("saws.db")
		require.NoError(t, err)
		assert.False(t, stat.IsDir())

		require.NoError(t, os.Remove("saws.db"))
	})

	t.Run("should add image row", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		err := table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		image, err := table.GetByID("image123")
		require.NoError(t, err)

		assert.Equal(t, image.ID, "image123")
		assert.Equal(t, image.Location, "Nicaragua")
	})

	t.Run("should get list of image rows", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		err := table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:         "image456",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		rows, err := table.Get()
		require.NoError(t, err)

		assertContainsRowWithID(t, rows, "image123")
		assertContainsRowWithID(t, rows, "image456")
	})

	t.Run("should get rows sorted by created order", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		err := table.Save(db.Image{
			ID:        "image123",
			MimeType:  "jpg",
			Location:  "Nicaragua",
			CreatedAt: time.Now().Add(-time.Hour),
		})
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:        "image456",
			MimeType:  "jpg",
			Location:  "Nicaragua",
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		rows, err := table.Get(db.GetOpts{OrderDirection: db.DESC})
		require.NoError(t, err)

		require.Len(t, rows, 2)
		assert.Equal(t, "image456", rows[0].ID)
		assert.Equal(t, "image123", rows[1].ID)

		rows, err = table.Get()
		require.NoError(t, err)

		require.Len(t, rows, 2)
		assert.Equal(t, "image123", rows[0].ID)
		assert.Equal(t, "image456", rows[1].ID)
	})
}

type testTable struct {
	t *testing.T
	*db.ImageTable
}

func newTestTable(t *testing.T) testTable {
	require.NoFileExists(t, "saws.db")
	table, err := db.NewImageTable("")
	require.NoError(t, err)
	return testTable{t, table}
}

func (t *testTable) Close() error {
	_ = t.ImageTable.Close()
	return os.Remove("saws.db")
}

func assertContainsRowWithID(t *testing.T, imgs []db.Image, id string) {
	assert.Truef(t, slices.ContainsFunc(imgs, func(img db.Image) bool {
		return img.ID == id
	}), "does not contain row with id %s", id)
}
