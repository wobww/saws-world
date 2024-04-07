package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wobwainwwight/sa-photos/db"
	"github.com/wobwainwwight/sa-photos/image"
	"github.com/wobwainwwight/sa-photos/templates"

	"github.com/pkg/errors"
)

var imageDir = filepath.Join("saws_world_data", "image_uploads")
var dsn = "file:saws_world_data/saws.sqlite"

func main() {
	appTemplates, err := templates.GetTemplates()
	if err != nil {
		log.Fatalf("could not get app templates: %s", err.Error())
		return
	}

	is, err := image.NewStore(imageDir)
	if err != nil {
		log.Fatalf("could not setup image file store: %s", err.Error())
		return
	}

	table, err := db.NewImageTable(dsn)
	if err != nil {
		log.Fatalf("could not create image table: %s", err.Error())
		return
	}
	defer table.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		tmpl := appTemplates.Lookup(templates.Index)
		if tmpl == nil {
			log.Printf("%s template not found\n", templates.Index)
			return
		}

		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("GET /south-america", func(w http.ResponseWriter, r *http.Request) {
		tmpl := appTemplates.Lookup(templates.SouthAmerica)
		if tmpl == nil {
			log.Printf("%s template not found\n", templates.SouthAmerica)
			return
		}

		type imageData struct {
			Title     string
			ImageURLs []string
		}

		imgData := imageData{
			Title: "South America 2023/24!",
		}

		imgs, err := table.Get()
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, img := range imgs {
			log.Println(img.ID)
			imgData.ImageURLs = append(imgData.ImageURLs, fmt.Sprintf("images/%s", img.ID))
		}

		tmpl.Execute(w, imgData)
	})

	mux.HandleFunc("POST /south-america", func(w http.ResponseWriter, r *http.Request) {
		log.Println("File Upload Endpoint Hit")
		log.Println(r.Method, r.RequestURI)

		// Parse our multipart form, 10 << 20 specifies a maximum
		// upload of 10 MB files.
		r.ParseMultipartForm(10 << 20)

		file, header, err := r.FormFile("image")
		if err != nil {
			err = errors.Wrap(err, "error retrieving image")
			log.Println(err)
			return
		}
		defer file.Close()

		log.Printf("Uploaded File: %+v\n", header.Filename)
		log.Printf("File Size: %+v\n", header.Size)
		log.Printf("MIME Header: %+v\n", header.Header)

		img, err := is.Save(file)
		if err != nil {
			err = fmt.Errorf("could not save image file: %w", err)
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = table.Save(db.Image{
			ID:         img.ID,
			MimeType:   img.MimeType,
			Location:   "",
			UploadedAt: time.Now(),
		})
		if err != nil {
			err = fmt.Errorf("could not save image %s to table: %w", img.ID, err)
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Location", fmt.Sprintf("%s/images/%s", r.URL.Path, img.ID))
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("GET /images/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Println("get image", id)

		img, err := table.GetByID(id)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ext, err := image.GetExtFromMimeType(img.MimeType)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filePath := filepath.Join(imageDir, img.ID+string(ext))

		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", img.MimeType)
		w.Write(fileBytes)
	})

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    "localhost:8080",
		Handler: mux,
	}
	go func() {
		log.Println("running at localhost:8080")
		err = srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not start server: %s", err.Error())
		}
	}()

	sig := <-signalCh
	log.Printf("Received signal: %v\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v\n", err)
	}

	log.Println("Server shutdown gracefully")
}
