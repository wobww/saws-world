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
		errorOut(err)
		return
	}

	if inf.IsDir() {
		fmt.Fprintf(os.Stdout, "uploading files in: %s\n", os.Args[1])
		err = uploadImagesInDir(os.Args[1])
	} else {
		err = uploadFile(os.Args[1])
	}

	if err != nil {
		errorOut(err)
	}
}

func uploadImagesInDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := filepath.Ext(e.Name())
		if ext == ".jpeg" || ext == ".jpg" {
			path := filepath.Join(dir, e.Name())

			f, err := os.Open(path)
			if err != nil {
				return err
			}
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
	req, err := http.NewRequest(http.MethodPost, "https://saws.world/images", f)
	if err != nil {
		return err
	}
	req.SetBasicAuth(os.Getenv("SAWS_USER"), os.Getenv("SAWS_PASSWORD"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not upload %s : %w", path, err)
	}

	if resp.StatusCode < 400 {
		fmt.Fprintf(os.Stdout, "%d %s %s\n", resp.StatusCode, path, resp.Header.Get("Location"))
	} else {
		return fmt.Errorf("%d %s", resp.StatusCode, path)
	}
	return nil
}

func errorOut(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	os.Exit(1)
}
