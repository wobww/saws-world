package image_test

import (
	"bytes"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/image"
	"github.com/wobwainwwight/sa-photos/image/imagetest"
)

func TestImageStore(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	checker := testFileChecker{
		root: filepath.Join(wd, "imagetest"),
	}
	store, err := image.NewImageFileStore(checker.root)
	require.NoError(t, err)

	t.Run("should save jpeg with hash name", func(t *testing.T) {
		img := checker.testFileSaved(t, store, imagetest.FishJPEG(), "6a14a3595a01.jpeg")
		assert.Equal(t, 318, img.Width)
		assert.Equal(t, 159, img.Height)

		t.Run("should set created time to 0 unix for jpegs without exif", func(t *testing.T) {
			unix0 := time.Unix(0, 0)
			assert.WithinDuration(t, unix0, img.Created, time.Second)
		})
	})

	t.Run("should get exif created time", func(t *testing.T) {
		img := checker.testFileSaved(t, store, imagetest.NYJPEG(), "2cd311b83027.jpeg")
		assert.Equal(t, 1089, img.Width)
		assert.Equal(t, 722, img.Height)

		expectedTime, err := time.Parse(time.RFC1123, "Fri, 27 Oct 2023 13:48:13 CST")
		require.NoError(t, err)
		assert.WithinDuration(t, expectedTime, img.Created, time.Second)

		assert.Equal(t, "GfgNDYIneId/eHi6eGeIg1egcDcK", img.ThumbHash)
	})

	t.Run("should save png", func(t *testing.T) {
		img := checker.testFileSaved(t, store, imagetest.PlanePNG(), "33f9c0515ccb.png")
		assert.Equal(t, 975, img.Width)
		assert.Equal(t, 333, img.Height)
	})

}

func TestResize(t *testing.T) {
	targetHeight := 500

	t.Run("should scale up to max height", func(t *testing.T) {
		w := image.ResizeWidth(75, 250, targetHeight)
		assert.Equal(t, 150, w)
	})

	t.Run("should scale down to max height", func(t *testing.T) {
		w := image.ResizeWidth(250, 1000, targetHeight)
		assert.Equal(t, 125, w)
	})

}

type testFileChecker struct {
	root string
}

func (f testFileChecker) testFileSaved(t *testing.T, store image.ImageFileStore, imageFile fs.File, expectedFileName string) image.Image {
	imgBytes, err := io.ReadAll(imageFile)
	require.NoError(t, err)

	saved, err := store.Save(bytes.NewReader(imgBytes))
	require.NoError(t, err)

	expectedPath := filepath.Join(f.root, expectedFileName)
	require.FileExists(t, expectedPath)
	require.Equal(t, expectedFileName, saved.FileName)

	// file contents should be identical
	savedFile, err := os.Open(expectedPath)
	require.NoError(t, err)
	buf, err := io.ReadAll(savedFile)
	require.NoError(t, err)

	require.Equal(t, imgBytes, buf)

	// cleanup file created
	require.NoError(t, os.Remove(expectedPath))

	return saved
}

func BenchmarkSave(b *testing.B) {
	wd, err := os.Getwd()
	require.NoError(b, err)

	root := filepath.Join(wd, "imagetest")
	store, err := image.NewImageFileStore(root)
	require.NoError(b, err)

	ogImagePath := filepath.Join(root, "fish.jpg")
	imageFile, err := os.Open(ogImagePath)
	require.NoError(b, err)

	bts, err := io.ReadAll(imageFile)
	require.NoError(b, err)

	newFile := ""

	b.ResetTimer()

	for range b.N {
		buf := bytes.NewBuffer(bts)

		b.StartTimer()
		img, err := store.Save(buf)
		b.StopTimer()

		require.NoError(b, err)
		newFile = img.FileName
	}
	//delete resulting file
	require.NoError(b, os.Remove(filepath.Join(root, newFile)))
}
