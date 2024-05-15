package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/galdor/go-thumbhash"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

type FileStore interface {
	Save(file io.Reader) (Image, error)
	ReadFile(id string) ([]byte, error)
	Delete(id string) error
}

func NewImageFileStore(root string) (FileStoreImpl, error) {
	// create uploads folder if not already created
	if _, err := os.Stat(root); os.IsNotExist(err) {
		err = os.MkdirAll(root, 0755)
		if err != nil {
			return FileStoreImpl{}, err
		}

	}

	return FileStoreImpl{dir: root}, nil
}

type FileStoreImpl struct {
	dir string
}

type Image struct {
	ID        string
	FileName  string
	MimeType  string
	Width     int
	Height    int
	ThumbHash string
	Created   time.Time
	Lat       float64
	Long      float64
}

func (s FileStoreImpl) Save(file io.Reader) (Image, error) {
	h := sha256.New()
	fileBuf := new(bytes.Buffer)

	mw := io.MultiWriter(fileBuf, h)

	tee := io.TeeReader(file, mw)

	img, imgType, err := image.Decode(tee)
	if err != nil {
		return Image{}, fmt.Errorf("could not decode image config: %w", err)
	}

	_, err = io.Copy(mw, file)
	if err != nil {
		return Image{}, fmt.Errorf("could not save image file: %w", err)
	}

	var ed exifData
	var exifErr error
	if imgType == "jpeg" {
		exifBuf := bytes.NewBuffer(fileBuf.Bytes())
		ed, exifErr = getExifData(exifBuf)
	}

	thumbhash := thumbhash.EncodeImage(img)
	thumbhashStr := base64.StdEncoding.EncodeToString(thumbhash)

	id := fmt.Sprintf("%x", h.Sum(nil))[:12]

	fileName := fmt.Sprintf("%s.%s", id, imgType)

	dst, err := os.Create(filepath.Join(s.dir, fileName))
	if err != nil {
		return Image{}, fmt.Errorf("could not save image: %w", err)
	}

	defer dst.Close()

	_, err = io.Copy(dst, fileBuf)
	if err != nil {
		return Image{}, fmt.Errorf("could not save image: %w", err)
	}

	if exifErr != nil {
		fmt.Printf("error while getting exif for img %s: %s\n", id, exifErr.Error())
	}

	return Image{
		ID:        id,
		FileName:  fileName,
		MimeType:  "image/" + imgType,
		Width:     img.Bounds().Dx(),
		Height:    img.Bounds().Dy(),
		Created:   ed.dateCreated,
		ThumbHash: thumbhashStr,
		Lat:       ed.lat,
		Long:      ed.long,
	}, nil
}

type exifData struct {
	dateCreated time.Time
	lat         float64
	long        float64
}

func getExifData(r io.Reader) (exifData, error) {
	e, err := exif.Decode(r)
	if err != nil {
		return exifData{}, err
	}
	if e == nil {
		return exifData{}, fmt.Errorf("exif is nil")
	}
	ed := exifData{
		dateCreated: time.Unix(0, 0).UTC(),
	}
	imgCreated, err := e.DateTime()
	if err != nil {
		fmt.Println("could not get created time from exif")
	} else {
		fmt.Println("found created time", imgCreated)
		ed.dateCreated = imgCreated
	}
	ed.lat, ed.long, err = GetLatLongFromExif(e)
	return ed, err
}

func (s FileStoreImpl) Delete(id string) error {
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

func (s FileStoreImpl) checkFilename(id string) (string, bool, error) {
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

func (s FileStoreImpl) ReadFile(id string) ([]byte, error) {
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

func GetLatLongFromExif(ex *exif.Exif) (float64, float64, error) {
	latTag, err := ex.Get(exif.GPSLatitude)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get lat: %w", err)
	}
	latRefTag, err := ex.Get(exif.GPSLatitudeRef)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get lat direction: %w", err)
	}
	longTag, err := ex.Get(exif.GPSLongitude)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get long: %w", err)
	}
	longRefTag, err := ex.Get(exif.GPSLongitudeRef)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get long direction: %w", err)
	}

	lat, err := getDecimalDegreeFromTag(latTag, latRefTag)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get lat decimal degree: %w", err)
	}

	long, err := getDecimalDegreeFromTag(longTag, longRefTag)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get long decimal degree: %w", err)
	}

	return lat, long, nil
}

func getDecimalDegreeFromTag(degreeTag *tiff.Tag, directionTag *tiff.Tag) (float64, error) {
	rats := make([]big.Rat, 3)
	for i := 0; i < 3; i++ {
		r, err := degreeTag.Rat(i)
		if err != nil {
			return 0, fmt.Errorf("%w", err)
		}
		rats[i] = *r
	}

	f0, _ := rats[0].Float64()
	f1, _ := rats[1].Float64()
	f2, _ := rats[2].Float64()

	f := f0 + f1/60 + f2/3600

	direction, err := directionTag.StringVal()
	if err != nil {
		return 0, fmt.Errorf("could not get direction from tag: %w", err)
	}
	if direction == "N" || direction == "E" {
		return f, nil
	}
	if direction == "S" || direction == "W" {
		return -f, nil
	}
	return 0, fmt.Errorf("not a valid gps direction: %s", direction)
}
