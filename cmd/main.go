package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/wobwainwwight/sa-photos/image"
	"github.com/wobwainwwight/sa-photos/templates"

	"github.com/pkg/errors"
)

var imageDir = filepath.Join("saws_world_data", "image_uploads")

func main() {

	appTemplates, err := templates.GetTemplates()
	if err != nil {
		err = errors.Wrap(err, "could not get app templates")
		fmt.Println(err.Error())
	}

	is, err := image.NewStore(imageDir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		tmpl := appTemplates.Lookup(templates.Index)
		if tmpl == nil {
			fmt.Printf("%s template not found\n", templates.Index)
			return
		}

		tmpl.Execute(w, nil)
	})

	http.HandleFunc("GET /south-america", func(w http.ResponseWriter, r *http.Request) {
		tmpl := appTemplates.Lookup(templates.SouthAmerica)
		if tmpl == nil {
			fmt.Printf("%s template not found\n", templates.SouthAmerica)
			return
		}

		imageData := struct{ Title string }{
			Title: "South America 2023/24!",
		}

		filepath.WalkDir(imageDir, func(path string, d fs.DirEntry, err error) error {
			fmt.Println("path")
			return err
		})

		tmpl.Execute(w, imageData)
	})

	http.HandleFunc("POST /south-america", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("File Upload Endpoint Hit")
		fmt.Println(r.Method, r.RequestURI)

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

		fileName, err := is.Save(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Location", fmt.Sprintf("%s/%s", r.URL.Path, fileName))
		w.WriteHeader(http.StatusCreated)

	})
	http.Handle("/static", http.FileServer(http.Dir("static")))

	fmt.Println("running at localhost:8080")

	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		fmt.Println(err.Error())
	}

}
