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

type Saver interface {
	Save(r io.Reader) (Image, error)
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
	ID       string
	FileName string
	MimeType string
}

func (s Store) Save(file io.Reader) (Image, error) {
	h := sha256.New()
	fileBuf := new(bytes.Buffer)

	extBuf := make([]byte, 512)
	_, err := file.Read(extBuf)
	if err != nil {
		return Image{}, err
	}

	ext, mimeType, err := assertImageType(extBuf)
	if err != nil {
		return Image{}, err
	}

	mw := io.MultiWriter(fileBuf, h)
	_, err = mw.Write(extBuf)
	if err != nil {
		return Image{}, err
	}

	_, err = io.Copy(mw, file)
	if err != nil {
		return Image{}, errors.Wrap(err, "could not save image")
	}

	id := fmt.Sprintf("%x", h.Sum(nil))[:12]

	fileName := id + string(ext)

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
		MimeType: mimeType,
	}, nil
}

func assertImageType(b []byte) (imgExt, string, error) {
	mimeType := http.DetectContentType(b)
	imgExt, err := GetExtFromMimeType(mimeType)
	if err != nil {
		return "", "", err
	}
	return imgExt, mimeType, nil
}

func GetExtFromMimeType(mimeType string) (imgExt, error) {
	switch mimeType {
	case "image/jpeg":
		return jpgExt, nil
	case "image/png":
		return pngExt, nil
	}
	return "", errors.New("image is not jpg or png")
}
