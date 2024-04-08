package image

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func NewImageFileStore(root string) (ImageFileStore, error) {
	// create uploads folder if not already created
	if _, err := os.Stat(root); os.IsNotExist(err) {
		err = os.MkdirAll(root, 0755)
		if err != nil {
			return ImageFileStore{}, err
		}

	}

	return ImageFileStore{dir: root}, nil
}

type ImageFileStore struct {
	dir string
}

type Image struct {
	ID       string
	FileName string
	MimeType string
	Width    int
	Height   int
}

func (s ImageFileStore) Save(file io.Reader) (Image, error) {
	h := sha256.New()
	fileBuf := new(bytes.Buffer)

	mw := io.MultiWriter(fileBuf, h)

	tee := io.TeeReader(file, mw)

	conf, imgType, err := image.DecodeConfig(tee)
	if err != nil {
		return Image{}, fmt.Errorf("could not decode image config: %w", err)
	}

	_, err = io.Copy(mw, file)
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	id := fmt.Sprintf("%x", h.Sum(nil))[:12]

	fileName := fmt.Sprintf("%s.%s", id, imgType)

	dst, err := os.Create(filepath.Join(s.dir, fileName))
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	defer dst.Close()

	_, err = io.Copy(dst, fileBuf)
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	return Image{
		ID:       id,
		FileName: fileName,
		MimeType: "image/" + imgType,
		Width:    conf.Width,
		Height:   conf.Height,
	}, nil
}

func (s ImageFileStore) Delete(id string) error {
	filename, ok, err := s.checkFilename(id)
	if err != nil {
		return fmt.Errorf("could not delete image %s: %w", id, err)
	}

	if !ok {
		return fmt.Errorf("could not find image file with id %s", id)
	}

	err = os.Remove(filepath.Join(s.dir, filename))
	if err != nil {
		return fmt.Errorf("could not remove %s: %w", filename, err)
	}
	return nil
}

func (s ImageFileStore) checkFilename(id string) (string, bool, error) {
	dir, err := os.ReadDir(s.dir)
	if err != nil {
		return "", false, fmt.Errorf("could not read dir on filename check %s: %w", id, err)
	}
	for _, f := range dir {
		if !f.IsDir() && strings.Contains(f.Name(), id) {
			return f.Name(), true, nil
		}
	}
	return "", false, nil
}

func (s ImageFileStore) ReadFile(id string) ([]byte, error) {
	filename, ok, err := s.checkFilename(id)
	if err != nil {
		return nil, fmt.Errorf("could not get image file %s: %w", id, err)
	}
	if !ok {
		return nil, notFoundError{id}
	}
	return os.ReadFile(filepath.Join(s.dir, filename))
}

func IsNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

type notFoundError struct {
	id string
}

func (n notFoundError) Error() string {
	return fmt.Sprintf("image file not found: %s", n.id)
}

func ResizeWidth(width, height, targetHeight int) int {
	x := float64(targetHeight) / float64(height)

	return int(float64(width) * x)
}
