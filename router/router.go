package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/wobwainwwight/sa-photos/db"
	"github.com/wobwainwwight/sa-photos/geocode"
	"github.com/wobwainwwight/sa-photos/image"
	"googlemaps.github.io/maps"
)

type Services struct {
	ImageFileStore image.FileStore
	Templates      *template.Template
	ImageTable     *db.ImageTable
	MapsClient     *maps.Client
}

type Options struct {
	IncludeIndexPage bool
	Admins           []string
}

func NewRouter(svc Services, opts Options) Router {
	mux := http.NewServeMux()

	ro := Router{
		ServeMux: mux,
		Services: svc,
		Options:  opts,
	}

	mux.HandleFunc("GET /{$}", ro.index)
	mux.HandleFunc("GET /south-america", ro.southAmerica)
	mux.HandleFunc("GET /south-america/images/list", ro.southAmericaList)
	mux.HandleFunc("GET /south-america/images/{id}", ro.southAmericaImage)
	mux.HandleFunc("PUT /south-america/images", ro.putImages)
	mux.HandleFunc("POST /images", ro.postImage)
	mux.HandleFunc("GET /images/{id}", ro.getImage)
	mux.HandleFunc("PATCH /images/{id}", ro.patchImage)
	mux.HandleFunc("DELETE /images/{id}", ro.deleteImage)
	mux.HandleFunc("GET /api/images/{id}", ro.apiGetImage)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return ro
}

type Router struct {
	*http.ServeMux
	Services
	Options
}

func (ro *Router) index(w http.ResponseWriter, r *http.Request) {
	if !ro.IncludeIndexPage {
		http.Redirect(w, r, "/south-america", http.StatusFound)
		return
	}

	tmpl := ro.Templates.Lookup("index.html")
	if tmpl == nil {
		log.Println("index.html template not found")
		return
	}

	tmpl.Execute(w, nil)
}

func (ro *Router) southAmerica(w http.ResponseWriter, r *http.Request) {
	countriesParam := r.URL.Query().Get("countries")

	var countries []string
	if len(countriesParam) > 0 {
		countries = strings.Split(countriesParam, ",")
	}

	tmpl := ro.Templates.Lookup("south-america.html")
	if tmpl == nil {
		log.Println("south-america.html template not found")
		return
	}

	opts := db.GetListOpts{
		Countries: countries,
		Order:     db.ASC,
	}
	order := r.URL.Query().Get("order")
	if order == "latest" {
		opts.Order = db.DESC
	}

	var previousCursor string
	var nextCursor string
	var imgs []db.Image
	var err error
	if !r.URL.Query().Has("jumpTo") {
		list, gerr := ro.ImageTable.GetList(
			db.WithOrder(opts.Order),
			db.WithCountries(countries...),
		)

		imgs = list.Images
		err = gerr
		nextCursor = list.Cursor.EncodedString()

	} else {
		jumpTo, err := ro.ImageTable.GetByID(r.URL.Query().Get("jumpTo"))
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reversedOrder := reverseOrder(opts.Order)
		pc, err := db.NewCursor(db.GetListOpts{
			Order:        reversedOrder,
			Countries:    countries,
			ExclStartKey: jumpTo.ID,
			Limit:        6,
		})
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prevList, err := ro.ImageTable.GetList(db.WithCursor(pc))
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		nc, err := db.NewCursor(db.GetListOpts{
			Order:        opts.Order,
			Countries:    countries,
			ExclStartKey: jumpTo.ID,
			Limit:        6,
		})
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nextList, err := ro.ImageTable.GetList(db.WithCursor(nc))
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slices.Reverse(prevList.Images)
		imgs = append(prevList.Images, jumpTo)
		imgs = append(imgs, nextList.Images...)
		nextCursor = nextList.Cursor.EncodedString()
		previousCursor = prevList.Cursor.EncodedString()
	}

	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imgPage := ImagesPage{}

	deleteEnabled := detemineIsAdmin(r, ro.Admins)

	if order == "latest" {
		imgPage.OrderBy = "latest"
	} else {
		imgPage.OrderBy = "oldest"
	}

	imgPage.Images = ToImageListItems(imgs, deleteEnabled, previousCursor, nextCursor)

	countryFilters := NewCountryFilters()
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

}

func (ro *Router) southAmericaList(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	pagination := r.URL.Query().Get("pagination")
	if pagination != "reverse" {
		pagination = "forward"
	}

	tmpl := ro.Templates.Lookup("image-list-items")
	if tmpl == nil {
		log.Println("image-list-items template not found")
		return
	}

	list, err := ro.ImageTable.GetList(
		db.WithCursorStr(cursor),
	)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	deleteEnabled := detemineIsAdmin(r, ro.Admins)

	il := ImagesPage{}

	if pagination == "reverse" {
		slices.Reverse(list.Images)
		il.Images = ToImageListItems(list.Images, deleteEnabled, list.Cursor.EncodedString(), "")
	} else {
		il.Images = ToImageListItems(list.Images, deleteEnabled, "", list.Cursor.EncodedString())
	}

	err = tmpl.Execute(w, il)
	if err != nil {
		log.Println(err.Error())
	}
}

func (ro *Router) southAmericaImage(w http.ResponseWriter, r *http.Request) {
	tmpl := ro.Templates.Lookup("south-america-image.html")
	if tmpl == nil {
		log.Println("south-america-image.html template not found")
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")

	img, err := ro.ImageTable.GetByID(id)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := ImagePage{
		ID:        img.ID,
		Title:     fmt.Sprintf("South America %s", img.ID),
		ImageURL:  fmt.Sprintf("/images/%s", img.ID),
		Width:     img.Width,
		Height:    img.Height,
		ThumbHash: img.ThumbHash,
	}

	prev, err := ro.ImageTable.GetList(db.WithDescOrder(), db.WithLimit(1), db.WithExclStartKey(img.ID))
	if err != nil {
		log.Println(err.Error())
	} else if len(prev.Images) == 1 {
		data.PrevURL = fmt.Sprintf("/south-america/images/%s", prev.Images[0].ID)
	}

	next, err := ro.ImageTable.GetList(db.WithAscOrder(), db.WithLimit(1), db.WithExclStartKey(img.ID))
	if err != nil {
		log.Println(err.Error())
	} else if len(next.Images) == 1 {
		data.NextURL = fmt.Sprintf("/south-america/images/%s", next.Images[0].ID)
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (ro *Router) putImages(w http.ResponseWriter, r *http.Request) {
	canDelete := detemineIsAdmin(r, ro.Admins)

	mr, err := r.MultipartReader()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := []ImageListItem{}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		log.Println(part.FileName())

		defer part.Close()

		log.Printf("uploaded File: %+v\n", part.FileName())
		log.Printf("MIME header: %+v\n", part.Header)

		img, err := ro.saveImage(part)
		if err == db.DuplicateImage {
			log.Println("dupe image")
			continue
		}

		if err != nil {
			log.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		items = append(items, ToImageListItem(img, canDelete))
	}

	w.WriteHeader(http.StatusCreated)

	tmpl := ro.Templates.Lookup("image-list-items")
	tmpl.Execute(w, ImagesPage{Images: items})

}

func (ro *Router) saveImage(imageFile io.Reader) (db.Image, error) {
	img, err := ro.ImageFileStore.Save(imageFile)
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

		if ro.MapsClient != nil {
			res, err := ro.MapsClient.Geocode(context.Background(), &maps.GeocodingRequest{
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

	err = ro.ImageTable.Save(dbImg)
	if err == db.DuplicateImage {
		return db.Image{}, err
	}
	if err != nil {
		err = fmt.Errorf("could not save image %s to table: %w", img.ID, err)
		return db.Image{}, err
	}
	return dbImg, nil
}

func (ro *Router) postImage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	img, err := ro.saveImage(r.Body)
	if err != nil {
		var exists image.ErrExist
		if errors.As(err, &exists) {
			w.Header().Add("Location", fmt.Sprintf("/images/%s", exists.ID))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		log.Print(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Location", fmt.Sprintf("/images/%s", img.ID))
	w.WriteHeader(http.StatusCreated)
	log.Println("created image: ", img.ID)
}

func (ro *Router) getImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	fileBytes, err := ro.ImageFileStore.ReadFile(id)
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
}

func (ro *Router) patchImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	imgPatch := db.Image{}

	dec := json.NewDecoder(r.Body)

	err := dec.Decode(&imgPatch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imgPatch.ID = id
	err = ro.ImageTable.Save(imgPatch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ro *Router) deleteImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := ro.ImageTable.Delete(id)
	if err != nil {
		msg := fmt.Sprintf("could not delete image %s from table: %s", id, err.Error())
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	err = ro.ImageFileStore.Delete(id)
	if err != nil {
		msg := fmt.Sprintf("could not delete image file %s: %s", id, err.Error())
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ro *Router) apiGetImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	img, err := ro.ImageTable.GetByID(id)
	if err != nil {
		if err == db.NotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		msg := fmt.Sprintf("could not get image %s from table: %s", id, err.Error())
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	enc := json.NewEncoder(w)
	err = enc.Encode(img)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func printJSON(d any) {
	b, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		fmt.Println("could not print json")
	}
	fmt.Print(string(b))
}

type ImagesPage struct {
	OrderBy        string
	CountryFilters []CountryFilter
	Images         []ImageListItem
	UploadEnabled  bool
}

type ImagePage struct {
	ID        string
	Title     string
	ImageURL  string
	Width     int
	Height    int
	ThumbHash string
	PrevURL   string
	NextURL   string
}

type CountryFilter struct {
	Value   string
	Display string
	Checked bool
}

func NewCountryFilters() []CountryFilter {
	return []CountryFilter{
		{"United States", "United States 🇺🇸", false},
		{"Chile", "Chile 🇨🇱", false},
		{"Argentina", "Argentina 🇦🇷", false},
		{"Bolivia", "Bolivia 🇧🇴", false},
		{"Peru", "Peru 🇵🇪", false},
		{"Colombia", "Colombia 🇨🇴", false},
		{"Costa Rica", "Costa Rica 🇨🇷", false},
		{"Nicaragua", "Nicaragua 🇳🇮", false},
	}
}

type ImageListItem struct {
	ID            string
	Width         int
	Height        int
	URL           string
	ImageURL      string
	Thumbhash     string
	DeleteEnabled bool

	GetPreviousPage bool
	PreviousCursor  string

	GetNextPage bool
	NextCursor  string
}

func ToImageListItems(imgs []db.Image, deleteEnabled bool, previousCursor string, nextCursor string) []ImageListItem {
	imgItems := make([]ImageListItem, len(imgs))
	for i, img := range imgs {
		il := ToImageListItem(img, deleteEnabled)

		if i == 0 && len(previousCursor) > 0 {
			il.GetPreviousPage = true
			il.PreviousCursor = previousCursor
		}

		if i == len(imgs)-1 && len(nextCursor) > 0 {
			il.GetNextPage = true
			il.NextCursor = nextCursor
		}

		imgItems[i] = il
	}

	return imgItems
}

func ToImageListItem(img db.Image, deleteEnabled bool) ImageListItem {
	targetHeight := 350
	return ImageListItem{
		ID:            img.ID,
		Width:         image.ResizeWidth(img.Width, img.Height, targetHeight),
		Height:        targetHeight,
		URL:           fmt.Sprintf("/south-america/images/%s", img.ID),
		ImageURL:      fmt.Sprintf("/images/%s", img.ID),
		Thumbhash:     img.ThumbHash,
		DeleteEnabled: deleteEnabled,
	}
}

func reverseOrder(order db.Order) db.Order {
	if order == db.ASC {
		return db.DESC
	}
	return db.ASC
}

func detemineIsAdmin(r *http.Request, admins []string) bool {
	if len(admins) == 0 {
		return false
	}
	username, _, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return slices.Contains(admins, username)
}
