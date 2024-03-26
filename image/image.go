package image

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
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
	fileBuf := new(bytes.Buffer)

	buff := make([]byte, 512)
	_, err := file.Read(buff)
	if err != nil {
		return Image{}, err
	}

	ext, err := assertImageType(buff)
	if err != nil {
		return Image{}, err
	}

	_, err = h.Write(buff)
	if err != nil {
		return Image{}, err
	}
	_, err = fileBuf.Write(buff)
	if err != nil {
		return Image{}, err
	}

	mw := io.MultiWriter(fileBuf, h)

	_, err = io.Copy(mw, file)
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
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

func assertImageType(b []byte) (imgExt, error) {
	mimeType := http.DetectContentType(b)

	if mimeType == "image/jpeg" {
		return jpgExt, nil
	}
	if mimeType == "image/png" {
		return pngExt, nil
	}
	return "", errors.New("image is not jpg or png")
}
