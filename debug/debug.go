package debug

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func Logwd() {
	dir, err := os.Getwd()
	if err != nil {
		log.Printf("could not getwd: %s", err.Error())
		return
	}

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println(err.Error())
		} else {
			log.Println(path, d.IsDir())
		}
		return nil
	})
	if err != nil {
		log.Printf("could not walk dir: %s", err.Error())
	}
}
