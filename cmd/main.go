package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wobwainwwight/sa-photos/db"
	"github.com/wobwainwwight/sa-photos/geocode"
	"github.com/wobwainwwight/sa-photos/image"
	"github.com/wobwainwwight/sa-photos/templates"
	"googlemaps.github.io/maps"
)

func main() {
	imageDir := filepath.Join("saws_world_data", "image_uploads")
	dsn := "file:saws_world_data/saws.sqlite?_journal=WAL"
	apiKey, apiKeyOK := os.LookupEnv("MAPS_KEY")

	port, portOK := os.LookupEnv("PORT")
	if !portOK {
		port = "8080"
	}

	host, hostOK := os.LookupEnv("HOST")
	if !hostOK {
		host = "127.0.0.1"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	inclIndexEnv, inclIndexOK := os.LookupEnv("SAWS_INDEX")
	if !inclIndexOK {
		inclIndexEnv = "0"
	}

	includeIndexPage := inclIndexEnv == "1"

	password, passwordOK := os.LookupEnv("SAWS_PASSWORD")

	requireBasicAuth := requireBasicAuthMiddleware(passwordMiddlewareOpts{
		enabled:  passwordOK,
		password: password,
	})

	adminsEnv, adminsOK := os.LookupEnv("SAWS_ADMINS")
	admins := []string{}
	if adminsOK {
		admins = strings.Split(adminsEnv, ",")
	}

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

	var client *maps.Client
	if apiKeyOK {
		client, err = maps.NewClient(maps.WithAPIKey(apiKey))
		if err != nil {
			log.Printf("could not initialise maps client: %s\n", err.Error())
			client = nil
		} else {
			log.Println("maps client initialised")
		}
	}

	imgSaver := imageSaver{
		ifs:   &is,
		table: table,
		m:     client,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if !includeIndexPage {
			http.Redirect(w, r, "/south-america", http.StatusFound)
			return
		}

		tmpl := appTemplates.Lookup(templates.Index)
		if tmpl == nil {
			log.Printf("%s template not found\n", templates.Index)
			return
		}

		tmpl.Execute(w, nil)
	})

	mux.Handle("GET /south-america", requireBasicAuth(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countriesParam := r.URL.Query().Get("countries")

			var countries []string
			if len(countriesParam) > 0 {
				countries = strings.Split(countriesParam, ",")
			}

			tmpl := appTemplates.Lookup(templates.SouthAmerica)
			if tmpl == nil {
				log.Printf("%s template not found\n", templates.SouthAmerica)
				return
			}

			opts := db.GetOpts{
				Countries: countries,
				Page:      1,
			}
			order := r.URL.Query().Get("order")
			if order == "latest" {
				opts.OrderDirection = db.DESC
			}

			var imgs []db.Image
			var err error
			if !r.URL.Query().Has("jumpTo") {
				imgs, err = table.Get(opts)
			} else {
				jumpTo := r.URL.Query().Get("jumpTo")
				jumpToImg, err := table.GetByID(jumpTo)
				if err != nil {
					log.Println(err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				imgs = append(imgs, jumpToImg)
			}

			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			imgPage := imagesPage{
				Title: "South America 2023/24!",
			}

			deleteEnabled := determineCanDelete(r, admins)

			if order == "latest" {
				imgPage.OrderBy = "latest"
			} else {
				imgPage.OrderBy = "oldest"
			}

			imgPage.Images = toImageListItems(imgs, deleteEnabled, 1)

			countryFilters := []countryFilter{
				{"United States", "United States 🇺🇸", false},
				{"Chile", "Chile 🇨🇱", false},
				{"Argentina", "Argentina 🇦🇷", false},
				{"Bolivia", "Bolivia 🇧🇴", false},
				{"Peru", "Peru 🇵🇪", false},
				{"Colombia", "Colombia 🇨🇴", false},
				{"Costa Rica", "Costa Rica 🇨🇷", false},
				{"Nicaragua", "Nicaragua 🇳🇮", false},
			}
			for i, c := range countryFilters {
				if strings.Contains(countriesParam, c.Value) {
					countryFilters[i].Checked = true
				}
			}

			imgPage.CountryFilters = countryFilters

			err = tmpl.Execute(w, imgPage)
			if err != nil {
				log.Println(err.Error())
			}
		})))

	mux.Handle("GET /south-america/images/list", requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		countriesParam := r.URL.Query().Get("countries")
		page := r.URL.Query().Get("page")
		pageNo, err := strconv.Atoi(page)
		if err != nil {
			err := fmt.Errorf("invalid page param: %s\n", page)
			log.Printf(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var countries []string
		if len(countriesParam) > 0 {
			countries = strings.Split(countriesParam, ",")
		}

		tmpl := appTemplates.Lookup("image-list-items")
		if tmpl == nil {
			log.Printf("%s template not found\n", templates.SouthAmerica)
			return
		}

		opts := db.GetOpts{
			Countries: countries,
			Page:      pageNo,
		}
		order := r.URL.Query().Get("order")
		if order == "latest" {
			opts.OrderDirection = db.DESC
		}
		imgs, err := table.Get(opts)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		deleteEnabled := determineCanDelete(r, admins)

		type images struct {
			Images []imageListItem
		}
		il := images{
			Images: toImageListItems(imgs, deleteEnabled, pageNo),
		}
		err = tmpl.Execute(w, il)
		if err != nil {
			log.Println(err.Error())
		}
	})))

	mux.Handle("GET /south-america/images/{id}", requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			data.PrevURL = fmt.Sprintf("/south-america/images/%s", prev[0].ID)
		}

		next, err := table.GetNext(id)
		if err != nil {
			log.Println(err.Error())
		} else {
			data.NextURL = fmt.Sprintf("/south-america/images/%s", next[0].ID)
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})))

	mux.Handle("PUT /south-america/images", requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)

		canDelete := determineCanDelete(r, admins)

		mr, err := r.MultipartReader()
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		items := []imageListItem{}

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			log.Println(part.FileName())

			defer part.Close()

			log.Printf("uploaded File: %+v\n", part.FileName())
			log.Printf("MIME header: %+v\n", part.Header)

			img, err := imgSaver.saveImage(part)
			if err == db.DuplicateImage {
				log.Println("dupe image")
				continue
			}

			if err != nil {
				log.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			items = append(items, toImageListItem(img, canDelete))
		}

		w.WriteHeader(http.StatusCreated)

		tmpl := appTemplates.Lookup("image-list-items")
		type images struct {
			Images []imageListItem
		}
		imgs := images{items}
		tmpl.Execute(w, imgs)

	})))

	mux.Handle("POST /images", requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		img, err := imgSaver.saveImage(r.Body)
		if err != nil {
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		imageURL := fmt.Sprintf("/images/%s", img.ID)
		w.Header().Add("Location", imageURL)
		w.WriteHeader(http.StatusCreated)
		log.Println("created image: ", img.ID)
	})))

	mux.Handle("GET /images/{id}", requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

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

		w.Header().Add("Cache-Control", "private, max-age=2628288, immutable")
		w.Write(fileBytes)
	})))

	mux.Handle("PATCH /images/{id}",
		requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")

			type loc struct {
				Locality string `json:"locality"`
				Country  string `json:"country"`
			}

			dec := json.NewDecoder(r.Body)

			ll := loc{}
			err = dec.Decode(&ll)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			_, err = table.DB.Exec("UPDATE image SET locality = (?), country = (?) WHERE id = (?)", ll.Locality, ll.Country, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})))

	mux.Handle("DELETE /images/{id}",
		requireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		})))

	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("running at %s\n", addr)
		err = srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not start server: %s", err.Error())
		}
	}()

	sig := <-signalCh
	log.Printf("received signal: %v\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v\n", err)
	}

	log.Println("shutting down")
}

type imageSaver struct {
	ifs   *image.ImageFileStore
	table *db.ImageTable
	m     *maps.Client
}

func (is *imageSaver) saveImage(imageFile io.Reader) (db.Image, error) {
	img, err := is.ifs.Save(imageFile)
	if err != nil {
		err = fmt.Errorf("could not save image file: %w", err)
		return db.Image{}, err
	}

	dbImg := db.Image{
		ID:         img.ID,
		MimeType:   img.MimeType,
		Width:      img.Width,
		Height:     img.Height,
		ThumbHash:  img.ThumbHash,
		Lat:        img.Lat,
		Long:       img.Long,
		UploadedAt: time.Now(),
		CreatedAt:  img.Created,
	}

	if img.Lat != 0 && img.Long != 0 {

		if is.m != nil {
			res, err := is.m.Geocode(context.Background(), &maps.GeocodingRequest{
				LatLng: &maps.LatLng{Lat: img.Lat, Lng: img.Long},
			})
			if err != nil {
				log.Printf("could not geocode from %.6f, %.6f: %s\n", img.Lat, img.Long, err.Error())
			} else if len(res) == 0 {
				log.Printf("no results for geocode from %.6f, %.6f\n", img.Lat, img.Long)
			} else {
				dbImg.Locality, dbImg.Country, err = geocode.GetLocalityAndCountry(res)
				if err != nil {
					log.Println(err.Error())
				}
			}
		} else {
			log.Println("maps client not initialised")
		}

	}

	err = is.table.Save(dbImg)
	if err == db.DuplicateImage {
		return db.Image{}, err
	}
	if err != nil {
		err = fmt.Errorf("could not save image %s to table: %w", img.ID, err)
		return db.Image{}, err
	}
	return dbImg, nil
}

type imageListItem struct {
	ID            string
	Width         int
	Height        int
	URL           string
	ImageURL      string
	Thumbhash     string
	DeleteEnabled bool
	GetNextPage   bool
	NextPage      int
}

type countryFilter struct {
	Value   string
	Display string
	Checked bool
}

type imagesPage struct {
	Title          string
	OrderBy        string
	CountryFilters []countryFilter
	Images         []imageListItem
	UploadEnabled  bool
}

type Middleware func(http.Handler) http.Handler

type passwordMiddlewareOpts struct {
	enabled    bool
	password   string
	privateKey string
}

func requireBasicAuthMiddleware(opts passwordMiddlewareOpts) Middleware {
	return func(next http.Handler) http.Handler {
		if !opts.enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, password, ok := r.BasicAuth()
			if !ok || password != opts.password {
				w.Header().Add("WWW-Authenticate", `Basic realm="Access to saws.world"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func determineCanDelete(r *http.Request, admins []string) bool {
	if len(admins) == 0 {
		return false
	}
	username, _, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return slices.Contains(admins, username)
}

func toImageListItems(imgs []db.Image, deleteEnabled bool, pageNo int) []imageListItem {
	imgItems := make([]imageListItem, len(imgs))
	for i, img := range imgs {
		il := toImageListItem(img, deleteEnabled)
		if i == len(imgs)-1 {
			il.GetNextPage = true
			il.NextPage = pageNo + 1
		}
		imgItems[i] = il
	}

	return imgItems
}

func toImageListItem(img db.Image, deleteEnabled bool) imageListItem {
	targetHeight := 350
	return imageListItem{
		ID:            img.ID,
		Width:         image.ResizeWidth(img.Width, img.Height, targetHeight),
		Height:        targetHeight,
		URL:           fmt.Sprintf("/south-america/images/%s", img.ID),
		ImageURL:      fmt.Sprintf("/images/%s", img.ID),
		Thumbhash:     img.ThumbHash,
		DeleteEnabled: deleteEnabled,
	}
}
