package db_test

import (
	"database/sql"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
)

func TestDB(t *testing.T) {

	t.Run("should set up image table", func(t *testing.T) {
		require.NoFileExists(t, "test.db")
		d, err := sql.Open("sqlite3", "file:test.db")
		require.NoError(t, err)
		defer os.Remove("test.db")

		_, ok, err := db.CheckImageTableExists(d)
		require.NoError(t, err)
		require.False(t, ok)

		_, err = db.NewImageTable("file:test.db")
		require.NoError(t, err)

		tableInfo, ok, err := db.CheckImageTableExists(d)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "image", tableInfo.Name)

		require.NoError(t, os.Remove("test.db"))
	})

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
		require.NoFileExists(t, "saws.db")
		table, err := db.NewImageTable("")
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
		})
		require.NoError(t, err)

		image, err := table.GetByID("image123")
		require.NoError(t, err)

		assert.Equal(t, image.ID, "image123")
		assert.Equal(t, image.Location, "Nicaragua")

		require.NoError(t, os.Remove("saws.db"))
	})

	t.Run("should get list of image rows", func(t *testing.T) {
		require.NoFileExists(t, "saws.db")
		table, err := db.NewImageTable("")
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
		})
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:         "image456",
			MimeType:   "jpg",
			Location:   "Nicaragua",
			UploadedAt: time.Now(),
		})
		require.NoError(t, err)

		rows, err := table.Get()
		require.NoError(t, err)

		assertContainsRowWithID(t, rows, "image123")
		assertContainsRowWithID(t, rows, "image456")

		require.NoError(t, os.Remove("saws.db"))
	})
}

func assertContainsRowWithID(t *testing.T, imgs []db.Image, id string) {
	assert.Truef(t, slices.ContainsFunc(imgs, func(img db.Image) bool {
		return img.ID == id
	}), "does not contain row with id %s", id)
}
