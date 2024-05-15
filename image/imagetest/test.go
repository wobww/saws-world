package imagetest

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/wobwainwwight/sa-photos/image"
)

//go:embed dogs.jpg fish.jpg plane.png new-york.jpeg
var f embed.FS

// FishJPEG returns the test jpeg image of a fish
func FishJPEG() fs.File {
	return mustOpen("fish.jpg")
}

// NYJPEG return test jpeg image of New York
func NYJPEG() fs.File {
	return mustOpen("new-york.jpeg")
}

// PlanePNG returns the test jpeg image of a fish
func PlanePNG() fs.File {
	return mustOpen("plane.png")
}

// DogsJPEG returns test jpeg image of dogs
func DogsJPEG() fs.File {
	return mustOpen("dogs.jpg")
}

func mustOpen(name string) fs.File {
	imageFile, err := f.Open(name)
	if err != nil {
		panic(fmt.Sprintf("could not open file %s: %s", name, err.Error()))
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
	store     *image.FileStoreImpl
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

func (t *TestStore) Delete(id string) error {
	err := t.store.Delete(id)
	if err != nil {
		return err
	}
	t.removeFileName(id)
	return nil
}

func (t *TestStore) ReadFile(id string) ([]byte, error) {
	return t.store.ReadFile(id)
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

func (t *TestStore) removeFileName(name string) {
	i := slices.Index(t.fileNames, name)
	t.fileNames = slices.Delete(t.fileNames, i, i+1)
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
