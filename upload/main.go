package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: upload-imgs <dir / file>")
		return
	}
	inf, err := os.Stat(os.Args[1])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if inf.IsDir() {
		err = uploadImagesInDir(os.Args[1])
		if err != nil {
			fmt.Println(err.Error())
		}
	} else {
		uploadFile(os.Args[1])
	}
}

func uploadImagesInDir(dir string) error {
	fmt.Printf("uploading files in: %s\n", dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := filepath.Ext(e.Name())
		fmt.Println(ext)
		if ext == ".jpeg" || ext == ".jpg" {
			path := filepath.Join(dir, e.Name())

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			fmt.Println("uploading", path)
			resp, err := http.Post("http://localhost:8080/images", "image/jpeg", f)
			if err != nil {
				return err
			}
			fmt.Printf("posted file %s: %d\n", e.Name(), resp.StatusCode)
		}

	}
	return nil
}

func uploadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	fmt.Println("uploading", path)
	resp, err := http.Post("http://localhost:8080/images", "image/jpeg", f)
	if err != nil {
		return err
	}
	fmt.Printf("posted file %s: %d\n", path, resp.StatusCode)
	return nil
}
