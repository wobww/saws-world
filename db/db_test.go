package db_test

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
	"github.com/wobwainwwight/sa-photos/db/dbtest"
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
		table := dbtest.NewTestTable(t)
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
		table := dbtest.NewTestTable(t)
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

		list, err := table.GetList()
		require.NoError(t, err)

		assertContainsRowWithID(t, list.Images, "image123")
		assertContainsRowWithID(t, list.Images, "image456")
	})

	t.Run("should get rows sorted by created order", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
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

		list, err := table.GetList(db.WithDescOrder())
		require.Len(t, list.Images, 2)
		assert.Equal(t, "image456", list.Images[0].ID)
		assert.Equal(t, "image123", list.Images[1].ID)

		list, err = table.GetList()
		require.NoError(t, err)

		require.Len(t, list.Images, 2)
		assert.Equal(t, "image123", list.Images[0].ID)
		assert.Equal(t, "image456", list.Images[1].ID)
	})

	t.Run("should get previous from specific row", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		fromImg := givenImageCreatedAt(t, time.Now())
		imgs := []db.Image{
			givenImageCreatedAt(t, time.Now().Add(-5*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-4*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-3*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-2*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-1*time.Hour)),
			fromImg,
			givenImageCreatedAt(t, time.Now().Add(time.Hour)),
		}

		for _, img := range imgs {
			err := table.Save(img)
			require.NoError(t, err)
		}

		list, err := table.GetList(db.WithOpts(db.GetListOpts{
			Order:        db.DESC,
			ExclStartKey: fromImg.ID,
			Limit:        3,
		}))
		require.NoError(t, err)
		require.Len(t, list.Images, 3)

		assert.Equal(t, list.Images[0].ID, imgs[4].ID)
		assert.Equal(t, list.Images[1].ID, imgs[3].ID)
		assert.Equal(t, list.Images[2].ID, imgs[2].ID)

		// should return remaining rows
		list, err = table.GetList(db.WithOpts(db.GetListOpts{
			Order:        db.DESC,
			ExclStartKey: list.Images[2].ID,
			Limit:        3,
		}))
		require.NoError(t, err)
		require.Len(t, list.Images, 2)

		assert.Equal(t, list.Images[0].ID, imgs[1].ID)
		assert.Equal(t, list.Images[1].ID, imgs[0].ID)

		// should return no rows that are left
		list, err = table.GetList(db.WithOpts(db.GetListOpts{
			Order:        db.DESC,
			ExclStartKey: list.Images[1].ID,
			Limit:        3,
		}))
		require.NoError(t, err)
		assert.Empty(t, list.Images)
	})

	t.Run("should get next rows", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		fromImg := givenImageCreatedAt(t, time.Now())
		imgs := []db.Image{
			givenImageCreatedAt(t, time.Now().Add(-3*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-2*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(-time.Hour)),
			fromImg,
			givenImageCreatedAt(t, time.Now().Add(time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(2*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(3*time.Hour)),
			givenImageCreatedAt(t, time.Now().Add(4*time.Hour)),
		}

		for _, img := range imgs {
			err := table.Save(img)
			require.NoError(t, err)
		}

		list, err := table.GetList(db.WithOpts(db.GetListOpts{
			Order:        db.ASC,
			ExclStartKey: fromImg.ID,
			Limit:        2,
		}))
		require.NoError(t, err)
		require.Len(t, list.Images, 2)

		assert.Equal(t, list.Images[0].ID, imgs[4].ID)
		assert.Equal(t, list.Images[1].ID, imgs[5].ID)

		list, err = table.GetList(db.WithOpts(db.GetListOpts{
			Order:        db.ASC,
			ExclStartKey: list.Images[1].ID,
			Limit:        2,
		}))
		require.NoError(t, err)
		require.Len(t, list.Images, 2)

		assert.Equal(t, list.Images[0].ID, imgs[6].ID)
		assert.Equal(t, list.Images[1].ID, imgs[7].ID)
	})

	t.Run("should filter by country with cursor", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		imgs := dbtest.GivenSaved(t, table, dbtest.SpaceByHour([]db.Image{
			givenImageInLocale(t, "United States", "New York"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Argentina", "Buenos Aires"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Puerto Natales"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Bolivia", "La Paz"),
			givenImageInLocale(t, "Chile", "Santiago"),
		})...)

		tests := []struct {
			Name            string
			StartingFrom    string
			Countries       []string
			Limit           int
			Direction       db.Order
			ExpectedIndexes []int
		}{
			// Ascending
			{"no country filter", "", []string{}, 100, db.ASC, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
			{"excl start key", imgs[0].ID, []string{}, 100, db.ASC, []int{1, 2, 3, 4, 5, 6, 7, 8}},
			{"Chile asc from middle", imgs[4].ID, []string{"Chile"}, 3, db.ASC, []int{5, 6, 8}},
			{"Chile, Bolivia asc from middle", imgs[4].ID, []string{"Chile", "Bolivia"}, 4, db.ASC, []int{5, 6, 7, 8}},
			{"empty filter from middle", imgs[4].ID, []string{"Argentina"}, 3, db.ASC, []int{}},

			// Descending
			{"desc from top", "", []string{}, 100, db.DESC, []int{8, 7, 6, 5, 4, 3, 2, 1, 0}},
			{"desc w excl start key", imgs[8].ID, []string{}, 100, db.DESC, []int{7, 6, 5, 4, 3, 2, 1, 0}},
			{"US descend from middle", imgs[4].ID, []string{"United States"}, 100, db.DESC, []int{0}},
			{"Chile, Arg desc", "", []string{"Chile", "Argentina"}, 6, db.DESC, []int{8, 6, 5, 4, 3, 2}},

			// edge cases
			{"id not found asc", "id-not-here", []string{}, 100, db.ASC, []int{}},
			{"id not found desc", "id-not-here", []string{}, 100, db.DESC, []int{}},
		}

		for _, tt := range tests {
			t.Run(tt.Name, func(t *testing.T) {
				list, err := table.GetList(db.WithOpts(db.GetListOpts{
					Order:        tt.Direction,
					ExclStartKey: tt.StartingFrom,
					Limit:        tt.Limit,
					Countries:    tt.Countries,
				}))
				require.NoError(t, err)

				require.Len(t, list.Images, len(tt.ExpectedIndexes))

				for i, expi := range tt.ExpectedIndexes {
					exp := imgs[expi]
					act := list.Images[i]

					assert.Equalf(t, exp.ID, act.ID,
						"expected: %s, %s actual %s, %s", exp.Locality, exp.Country, act.Locality, act.Country)
				}
			})
		}
	})

	t.Run("should work with cursor", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		imgs := dbtest.GivenSaved(t, table, dbtest.SpaceByHour([]db.Image{
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "United States", "New York"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
			givenImageInLocale(t, "Chile", "Santiago"),
		})...)

		cursor, err := db.NewCursor(db.GetListOpts{
			Order:     db.ASC,
			Countries: []string{"Chile"},
			Limit:     2,
		})
		require.NoError(t, err)

		list, err := table.GetList(db.WithCursorStr(cursor.String()))
		require.NoError(t, err)

		require.Len(t, list.Images, 2)
		assert.Equal(t, imgs[0].ID, list.Images[0].ID)
		assert.Equal(t, imgs[1].ID, list.Images[1].ID)

		list, err = table.GetList(db.WithCursor(list.Cursor))
		require.NoError(t, err)
		require.Len(t, list.Images, 2)
		assert.Equal(t, imgs[2].ID, list.Images[0].ID)
		assert.Equal(t, imgs[4].ID, list.Images[1].ID)

		list, err = table.GetList(db.WithCursorStr(list.Cursor.String()))
		require.NoError(t, err)
		require.Len(t, list.Images, 2)
		assert.Equal(t, imgs[5].ID, list.Images[0].ID)
		assert.Equal(t, imgs[6].ID, list.Images[1].ID)
	})

	t.Run("should get all localities", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		countryLocalities := map[string][]string{"United States": {"New York", "Washington DC", "Los Angeles"},
			"Wales":     {"Cardiff", "Swansea", "Newport"},
			"Argentina": {"San Carlos de Bariloche", "Mendoza"},
		}

		givenCountriesAndLocalities(t, table, countryLocalities)

		l, err := table.GetLocalities()
		require.NoError(t, err)

		for k, v := range countryLocalities {
			assertContainsLocality(t, l, k, v)
		}

	})

	t.Run("should upsert", func(t *testing.T) {
		table := dbtest.NewTestTable(t)
		defer table.Close()

		t.Run("country", func(t *testing.T) {
			img := dbtest.GivenImage(t)
			img.Country = "Wales"

			err := table.Save(img)
			require.NoError(t, err)

			upserted := img
			upserted.Country = "UK"
			assert.NotEqual(t, upserted, img)

			err = table.Save(upserted)
			require.NoError(t, err)

			fetched, err := table.GetByID(img.ID)
			require.NoError(t, err)

			assertImageEqual(t, upserted, fetched)
		})

		t.Run("locality", func(t *testing.T) {
			img := dbtest.GivenImage(t)
			img.Locality = "Brecon"

			err := table.Save(img)
			require.NoError(t, err)

			upserted := img
			upserted.Locality = "Abergavenny"
			assert.NotEqual(t, upserted, img)

			err = table.Save(upserted)
			require.NoError(t, err)

			fetched, err := table.GetByID(img.ID)
			require.NoError(t, err)

			assertImageEqual(t, upserted, fetched)
		})

	})

}

func givenCountriesAndLocalities(t *testing.T, it dbtest.TestTable, countryLocalities map[string][]string) {
	for country, localities := range countryLocalities {
		for _, l := range localities {
			err := it.Save(givenImageInLocale(t, country, l))
			require.NoError(t, err)
		}
	}
}

func givenImageCreatedAt(t *testing.T, tt time.Time) db.Image {
	i := dbtest.GivenImage(t)
	i.CreatedAt = tt
	return i
}

func givenImageInLocale(t *testing.T, country, locality string) db.Image {
	i := dbtest.GivenImage(t)
	i.Country = country
	i.Locality = locality
	return i
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

func assertContainsRowWithID(t *testing.T, imgs []db.Image, id string) {
	assert.Truef(t, slices.ContainsFunc(imgs, func(img db.Image) bool {
		return img.ID == id
	}), "does not contain row with id %s", id)
}

func assertImageEqual(t *testing.T, img1 db.Image, img2 db.Image) {
	eq := cmp.Equal(img1, img2, cmpopts.EquateApproxTime(time.Second))
	assert.True(t, eq)
}
