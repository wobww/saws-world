package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func main() {
	// create uploads folder if not already created
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		os.Mkdir("uploads", 0755)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("templates/index.gohtml")
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("File Upload Endpoint Hit")

		// Parse our multipart form, 10 << 20 specifies a maximum
		// upload of 10 MB files.
		r.ParseMultipartForm(10 << 20)

		file, header, err := r.FormFile("image")
		if err != nil {
			err = errors.Wrap(err, "error retrieving image")
			fmt.Println(err)
			return
		}
		defer file.Close()

		fmt.Printf("Uploaded File: %+v\n", header.Filename)
		fmt.Printf("File Size: %+v\n", header.Size)
		fmt.Printf("MIME Header: %+v\n", header.Header)

		dst, err := os.Create(filepath.Join("uploads", newFilename(header.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy the uploaded file to the new file
		_, err = io.Copy(dst, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "File %s uploaded successfully!", header.Filename)
	})

	fmt.Println("running at localhost:8080")

	http.ListenAndServe(":8080", nil)

}

func newFilename(fileName string) string {
	ext := filepath.Ext(fileName)

	// Get the current date in the desired format
	currentDate := time.Now().Format(time.RFC3339)

	// remove extension
	withoutExt := fileName[:len(fileName)-len(filepath.Ext(fileName))]

	// Construct the new filename with the date
	newFilename := fmt.Sprintf("%s-%s%s", withoutExt, currentDate, ext)

	return newFilename
}
