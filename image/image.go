package image

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func NewStore(root string) (Store, error) {
	// create uploads folder if not already created
	if _, err := os.Stat(root); os.IsNotExist(err) {
		err = os.MkdirAll(root, 0755)
		if err != nil {
			return Store{}, err
		}

	}

	return Store{dir: root}, nil
}

type Store struct {
	dir string
}

type imgExt string

const (
	jpgExt imgExt = ".jpg"
	pngExt imgExt = ".png"
)

type Image struct {
	FileName string
}

func (s Store) Save(file io.Reader) (Image, error) {
	h := sha256.New()
	extBuf := new(bytes.Buffer)
	fileBuf := new(bytes.Buffer)

	multiWriter := io.MultiWriter(h, extBuf, fileBuf)
	if _, err := io.Copy(multiWriter, file); err != nil {
		return Image{}, err
	}

	ext, err := assertImageType(extBuf)
	if err != nil {
		return Image{}, err
	}

	fileName := fmt.Sprintf("%x", h.Sum(nil))[:12] + string(ext)

	dst, err := os.Create(filepath.Join(s.dir, fileName))
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	defer dst.Close()

	_, err = io.Copy(dst, fileBuf)
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	return Image{FileName: fileName}, nil
}

func assertImageType(r io.Reader) (imgExt, error) {
	buf := new(bytes.Buffer)

	tee := io.TeeReader(r, buf)

	_, err := jpeg.DecodeConfig(tee)
	if err == nil {
		return jpgExt, nil
	}
	_, err = png.DecodeConfig(buf)
	if err == nil {
		return pngExt, nil
	}
	return "", errors.New("image is not jpg or png")
}
