package imagetest

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wobwainwwight/sa-photos/image"
)

//go:embed fish.jpg plane.png
var f embed.FS

// FishJPEG returns the test jpeg image of a fish
func FishJPEG() fs.File {
	imageFile, err := f.Open("fish.jpg")
	if err != nil {
		panic(fmt.Sprint("could not get fish jpeg ", err.Error()))
	}
	return imageFile
}

// PlanePNG returns the test jpeg image of a fish
func PlanePNG() fs.File {
	imageFile, err := f.Open("plane.png")
	if err != nil {
		panic(fmt.Sprint("could not get fish png ", err.Error()))
	}
	return imageFile
}

func NewStore() *TestStore {
	wd, err := os.Getwd()
	if err != nil {
		panic("could not create test store: " + err.Error())
	}
	st, err := image.NewImageFileStore(wd)
	if err != nil {
		panic("could not create test store: " + err.Error())
	}
	return &TestStore{
		store:     &st,
		dir:       wd,
		fileNames: []string{},
	}

}

type TestStore struct {
	store     *image.ImageFileStore
	dir       string
	fileNames []string
}

func (t *TestStore) Save(r io.Reader) (image.Image, error) {
	i, err := t.store.Save(r)
	if err != nil {
		return image.Image{}, err
	}

	t.fileNames = append(t.fileNames, i.FileName)
	return i, nil
}

// Close removes all files created by the teststore
func (t *TestStore) Close() {
	for _, fn := range t.fileNames {
		path := t.appendFileName(fn)
		err := os.Remove(path)
		if err != nil {
			fmt.Println("failed to remove file", path)
		}
	}
	t.fileNames = []string{}
}

func (t *TestStore) appendFileName(name string) string {
	return filepath.Join(t.dir, name)
}

func (t *TestStore) FileExistsWithName(name string) bool {
	stat, err := os.Stat(t.appendFileName(name))
	if os.IsNotExist(err) {
		return false
	}

	if stat.IsDir() {
		return false
	}

	return true
}
