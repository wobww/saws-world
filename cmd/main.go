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
var dsn = "file:saws_world_data/saws.sqlite?_journal=WAL"

func main() {
	appTemplates, err := templates.GetTemplates()
	if err != nil {
		log.Fatalf("could not get app templates: %s", err.Error())
		return
	}

	is, err := image.NewImageFileStore(imageDir)
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

		type imageListItem struct {
			Width      int
			Height     int
			URL        string
			TranslateX int
			TranslateY int
		}

		type imageData struct {
			Title  string
			Images []imageListItem
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

		log.Println("images len", len(imgs))
		imgData.Images = make([]imageListItem, len(imgs))
		targetHeight := 350
		for i, img := range imgs {
			w := image.ResizeWidth(img.Width, img.Height, targetHeight)

			translateX := 0
			if i != 0 {
				translateX = imgData.Images[i-1].Width
			}

			imgData.Images[i] = imageListItem{
				Width:      w,
				Height:     targetHeight,
				TranslateX: translateX,
				TranslateY: 0,
				URL:        fmt.Sprintf("images/%s?w=%d&h=%d", img.ID, w, targetHeight),
			}
		}

		err = tmpl.Execute(w, imgData)
		if err != nil {
			log.Println(err.Error())
		}
	})

	mux.HandleFunc("PUT /south-america/images", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Image file Upload Endpoint Hit")
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
			Width:      img.Width,
			Height:     img.Height,
			UploadedAt: time.Now(),
		})
		if err != nil {
			err = fmt.Errorf("could not save image %s to table: %w", img.ID, err)
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		imageURL := fmt.Sprintf("/images/%s", img.ID)
		w.Header().Add("Location", imageURL)
		w.WriteHeader(http.StatusCreated)

		tmpl := appTemplates.Lookup("image-list-item")
		tmpl.Execute(w, imageURL)

	})

	mux.HandleFunc("GET /images/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Println("GET image", id)

		fileBytes, err := is.ReadFile(id)
		if err != nil {
			code := http.StatusInternalServerError
			if image.IsNotFound(err) {
				code = http.StatusNotFound
			}
			log.Println(err.Error())
			http.Error(w, err.Error(), code)
			return
		}

		w.Write(fileBytes)
	})

	mux.HandleFunc("DELETE /images/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Printf("DELETE %s", id)
		err = table.Delete(id)
		if err != nil {
			msg := fmt.Sprintf("could not delete image %s from table: %s", id, err.Error())
			log.Println(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		err = is.Delete(id)
		if err != nil {
			msg := fmt.Sprintf("could not delete image file %s: %s", id, err.Error())
			log.Println(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

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
