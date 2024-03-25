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
		fishPath := filepath.Join(root, "fish.jpg")
		imageFile, err := os.Open(fishPath)
		require.NoError(t, err)

		saved, err := store.Save(imageFile)
		require.NoError(t, err)

		expected := "6a14a3595a01.jpg"
		expectedPath := filepath.Join(root, expected)
		require.FileExists(t, expectedPath)
		require.Equal(t, expected, saved.FileName)

		// file contents should be identical
		f, err := os.Open(expectedPath)
		require.NoError(t, err)
		buf, err := io.ReadAll(f)
		require.NoError(t, err)

		imageFile, err = os.Open(fishPath)
		require.NoError(t, err)
		expBuf, err := io.ReadAll(imageFile)
		require.NoError(t, err)

		require.Equal(t, buf, expBuf)

		// cleanup file created
		require.NoError(t, os.Remove(expectedPath))
	})

}
