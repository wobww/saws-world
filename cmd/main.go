package main

import (
	"context"
	"fmt"
	"io"
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

		opts := db.GetOpts{}
		if r.URL.Query().Get("order") == "latest" {
			opts.OrderDirection = db.DESC
		}
		imgs, err := table.Get(opts)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type imageListItem struct {
			Width      int
			Height     int
			URL        string
			ImageURL   string
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
				URL:        fmt.Sprintf("/south-america/images/%s", img.ID),
				ImageURL:   fmt.Sprintf("/images/%s?w=%d&h=%d", img.ID, w, targetHeight),
			}
		}

		err = tmpl.Execute(w, imgData)
		if err != nil {
			log.Println(err.Error())
		}
	})

	mux.HandleFunc("GET /south-america/images/{id}", func(w http.ResponseWriter, r *http.Request) {
		tmpl := appTemplates.Lookup("south-america-image")
		if tmpl == nil {
			log.Printf("%s template not found\n", "south-america-image")
			return
		}

		id := r.PathValue("id")

		img, err := table.GetByID(id)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type imagePage struct {
			Title     string
			ImageURL  string
			Width     int
			Height    int
			ThumbHash string
			PrevURL   string
			NextURL   string
		}

		data := imagePage{
			Title:     fmt.Sprintf("South America %s", img.ID),
			ImageURL:  fmt.Sprintf("/images/%s", img.ID),
			Width:     img.Width,
			Height:    img.Height,
			ThumbHash: img.ThumbHash,
		}

		prev, err := table.GetPrev(id)
		if err != nil {
			log.Println(err.Error())
		} else {
			data.PrevURL = fmt.Sprintf("/south-america/images/%s", prev.ID)
		}

		next, err := table.GetNext(id)
		if err != nil {
			log.Println(err.Error())
		} else {
			data.NextURL = fmt.Sprintf("/south-america/images/%s", next.ID)
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

		img, err := saveImage(is, table, file)
		if err != nil {
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

	mux.HandleFunc("POST /images", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		img, err := saveImage(is, table, r.Body)
		if err != nil {
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		imageURL := fmt.Sprintf("/images/%s", img.ID)
		w.Header().Add("Location", imageURL)
		w.WriteHeader(http.StatusCreated)
		log.Println("created image: ", img.ID)
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

	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

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

func saveImage(ifs image.ImageFileStore, table *db.ImageTable, imageFile io.Reader) (image.Image, error) {
	img, err := ifs.Save(imageFile)
	if err != nil {
		err = fmt.Errorf("could not save image file: %w", err)
		return image.Image{}, err
	}

	err = table.Save(db.Image{
		ID:         img.ID,
		MimeType:   img.MimeType,
		Location:   "",
		Width:      img.Width,
		Height:     img.Height,
		ThumbHash:  img.ThumbHash,
		UploadedAt: time.Now(),
		CreatedAt:  img.Created,
	})
	if err != nil {
		err = fmt.Errorf("could not save image %s to table: %w", img.ID, err)
		return image.Image{}, err
	}
	return img, nil
}
