package db_test

import (
	"os"
	"slices"
	"testing"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
)

func TestDB(t *testing.T) {

	t.Run("should setup saws.db if no arg", func(t *testing.T) {
		require.NoFileExists(t, "saws.sqlite")
		_, err := db.NewImageTable("")
		require.NoError(t, err)

		stat, err := os.Stat("saws.sqlite")
		require.NoError(t, err)
		assert.False(t, stat.IsDir())

		require.NoError(t, os.Remove("saws.sqlite"))
	})

	t.Run("should add image row", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		err := table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		image, err := table.GetByID("image123")
		require.NoError(t, err)

		assert.Equal(t, image.ID, "image123")
	})

	t.Run("should get list of image rows", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		err := table.Save(db.Image{
			ID:         "image123",
			MimeType:   "jpg",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:         "image456",
			MimeType:   "jpg",
			UploadedAt: time.Now(),
			CreatedAt:  time.Now(),
		})
		require.NoError(t, err)

		rows, err := table.GetList()
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
			CreatedAt: time.Now().Add(-time.Hour),
		})
		require.NoError(t, err)

		err = table.Save(db.Image{
			ID:        "image456",
			MimeType:  "jpg",
			CreatedAt: time.Now(),
		})
		require.NoError(t, err)

		rows, err := table.GetList(db.GetOpts{OrderDirection: db.DESC})
		require.NoError(t, err)

		require.Len(t, rows, 2)
		assert.Equal(t, "image456", rows[0].ID)
		assert.Equal(t, "image123", rows[1].ID)

		rows, err = table.GetList()
		require.NoError(t, err)

		require.Len(t, rows, 2)
		assert.Equal(t, "image123", rows[0].ID)
		assert.Equal(t, "image456", rows[1].ID)
	})

	t.Run("should get previous from specific row", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		fromImg := givenImageCreated(time.Now())
		imgs := []db.Image{
			givenImageCreated(time.Now().Add(-5 * time.Hour)),
			givenImageCreated(time.Now().Add(-4 * time.Hour)),
			givenImageCreated(time.Now().Add(-3 * time.Hour)),
			givenImageCreated(time.Now().Add(-2 * time.Hour)),
			givenImageCreated(time.Now().Add(-1 * time.Hour)),
			fromImg,
			givenImageCreated(time.Now().Add(time.Hour)),
		}

		for _, img := range imgs {
			err := table.Save(img)
			require.NoError(t, err)
		}

		rows, err := table.GetList(db.GetOpts{
			OrderDirection: db.DESC,
			FromRowID:      fromImg.ID,
			Limit:          3,
		})
		require.NoError(t, err)
		require.Len(t, rows, 3)

		assert.Equal(t, rows[0].ID, imgs[4].ID)
		assert.Equal(t, rows[1].ID, imgs[3].ID)
		assert.Equal(t, rows[2].ID, imgs[2].ID)

		// should return remaining rows
		rows, err = table.GetList(db.GetOpts{
			OrderDirection: db.DESC,
			FromRowID:      rows[2].ID,
			Limit:          3,
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, rows[0].ID, imgs[1].ID)
		assert.Equal(t, rows[1].ID, imgs[0].ID)

		// should return no rows that are left
		rows, err = table.GetList(db.GetOpts{
			OrderDirection: db.DESC,
			FromRowID:      rows[1].ID,
			Limit:          3,
		})
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("should get next rows", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		fromImg := givenImageCreated(time.Now())
		imgs := []db.Image{
			givenImageCreated(time.Now().Add(-3 * time.Hour)),
			givenImageCreated(time.Now().Add(-2 * time.Hour)),
			givenImageCreated(time.Now().Add(-time.Hour)),
			fromImg,
			givenImageCreated(time.Now().Add(time.Hour)),
			givenImageCreated(time.Now().Add(2 * time.Hour)),
			givenImageCreated(time.Now().Add(3 * time.Hour)),
			givenImageCreated(time.Now().Add(4 * time.Hour)),
		}

		for _, img := range imgs {
			err := table.Save(img)
			require.NoError(t, err)
		}

		rows, err := table.GetList(db.GetOpts{
			OrderDirection: db.ASC,
			FromRowID:      fromImg.ID,
			Limit:          2,
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, rows[0].ID, imgs[4].ID)
		assert.Equal(t, rows[1].ID, imgs[5].ID)

		rows, err = table.GetList(db.GetOpts{
			OrderDirection: db.ASC,
			FromRowID:      rows[1].ID,
			Limit:          2,
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, rows[0].ID, imgs[6].ID)
		assert.Equal(t, rows[1].ID, imgs[7].ID)
	})

	t.Run("should get all localities", func(t *testing.T) {
		table := newTestTable(t)
		defer table.Close()

		countryLocalities := map[string][]string{
			"United States": {"New York", "Washington DC", "Los Angeles"},
			"Wales":         {"Cardiff", "Swansea", "Newport"},
			"Argentina":     {"San Carlos de Bariloche", "Mendoza"},
		}

		givenCountriesAndLocalities(t, table, countryLocalities)

		l, err := table.GetLocalities()
		require.NoError(t, err)

		for k, v := range countryLocalities {
			assertContainsLocality(t, l, k, v)
		}

	})
}

func givenCountriesAndLocalities(t *testing.T, it testTable, countryLocalities map[string][]string) {
	for country, localities := range countryLocalities {
		for _, l := range localities {
			err := it.Save(givenImageInLocale(country, l))
			require.NoError(t, err)
		}
	}
}

func givenImageCreated(t time.Time) db.Image {
	return db.Image{
		ID:        gonanoid.Must(),
		CreatedAt: t,
	}
}

func givenImageInLocale(country, locality string) db.Image {
	return db.Image{
		ID:       gonanoid.Must(),
		Locality: locality,
		Country:  country,
	}
}

func assertContainsLocality(t *testing.T, ll []db.Locality, country string, localities []string) {
	found := false
	for _, l := range ll {
		if l.Country == country {
			found = true

			for _, lc := range localities {
				assert.Contains(t, l.Localities, lc)
			}
		}

	}
	assert.Truef(t, found, "country %s not found", country)

}

type testTable struct {
	t *testing.T
	*db.ImageTable
}

func newTestTable(t *testing.T) testTable {
	require.NoFileExists(t, "test-saws.sqlite")
	table, err := db.NewImageTable("file:test-saws.sqlite")
	require.NoError(t, err)
	return testTable{t, table}
}

func (t *testTable) Close() error {
	_ = t.ImageTable.Close()
	return os.Remove("test-saws.sqlite")
}

func assertContainsRowWithID(t *testing.T, imgs []db.Image, id string) {
	assert.Truef(t, slices.ContainsFunc(imgs, func(img db.Image) bool {
		return img.ID == id
	}), "does not contain row with id %s", id)
}
