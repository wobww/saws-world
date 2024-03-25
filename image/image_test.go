package image_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/image"
)

func TestImageStore(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	root := filepath.Join(wd, "test_images")
	store, err := image.NewStore(root)
	require.NoError(t, err)

	t.Run("should save jpeg with hash name", func(t *testing.T) {
		testFileSaved(t, store, root, "fish.jpg", "6a14a3595a01.jpg")
	})

	t.Run("should save png", func(t *testing.T) {
		testFileSaved(t, store, root, "plane.png", "33f9c0515ccb.png")
	})

}

func testFileSaved(t *testing.T, store image.Store, root, fileName, expectedFilename string) {
	ogImagePath := filepath.Join(root, fileName)
	imageFile, err := os.Open(ogImagePath)
	require.NoError(t, err)

	saved, err := store.Save(imageFile)
	require.NoError(t, err)

	expectedPath := filepath.Join(root, expectedFilename)
	require.FileExists(t, expectedPath)
	require.Equal(t, expectedFilename, saved.FileName)

	// file contents should be identical
	f, err := os.Open(expectedPath)
	require.NoError(t, err)
	buf, err := io.ReadAll(f)
	require.NoError(t, err)

	imageFile, err = os.Open(ogImagePath)
	require.NoError(t, err)
	expBuf, err := io.ReadAll(imageFile)
	require.NoError(t, err)

	require.Equal(t, buf, expBuf)

	// cleanup file created
	require.NoError(t, os.Remove(expectedPath))
}
