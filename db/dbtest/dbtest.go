package dbtest

import (
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
)

type TestTable struct {
	t *testing.T
	*db.ImageTable
}

func NewTestTable(t *testing.T) TestTable {
	require.NoFileExists(t, "test-saws.sqlite")
	table, err := db.NewImageTable("file:test-saws.sqlite")
	require.NoError(t, err)
	return TestTable{t, table}
}

func (t *TestTable) Close() error {
	_ = t.ImageTable.Close()
	return os.Remove("test-saws.sqlite")
}

func SpaceByHour(imgs []db.Image) []db.Image {
	for i := range imgs {
		imgs[i].CreatedAt = time.Now().Add(-time.Hour * time.Duration(len(imgs)-i))
	}
	return imgs
}

func GivenSaved(t *testing.T, table TestTable, images ...db.Image) []db.Image {
	for _, img := range images {
		err := table.Save(img)
		require.NoError(t, err)
	}
	return images
}

func GivenImage(t *testing.T) db.Image {
	img := db.Image{
		ID:         gonanoid.Must(),
		MimeType:   "image/jpeg",
		ThumbHash:  gonanoid.Must(),
		CreatedAt:  time.Now().UTC().Round(time.Second),
		UploadedAt: time.Now().UTC().Round(time.Second),
	}

	rand := rand.Float64()
	if rand > 0.5 {
		img.MimeType = "image/png"
	} else {
		img.MimeType = "image/jpeg"
	}
	img.Width = int(math.Round(rand * 100))
	img.Height = int(math.Round(rand * 120))
	img.Lat = rand * 100
	img.Long = rand * 100

	countries := []string{"United States", "Chile", "Argentina"}
	i := int(math.Round(rand*100)) % 3
	img.Country = countries[i]

	return img
}
