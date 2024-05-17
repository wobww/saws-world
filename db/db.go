package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "github.com/mattn/go-sqlite3"
)

var DuplicateImage = errors.New("duplicate image")
var NotFound = errors.New("image not found")

func NewImageTable(dsn string) (*ImageTable, error) {
	if len(strings.TrimSpace(dsn)) == 0 {
		dsn = "file:saws.sqlite"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	i := ImageTable{db}
	_, ok, err := i.CheckImageTableExists()
	if !ok {
		err = i.CreateImageTable()
		if err != nil {
			return nil, err
		}
	}
	return &i, nil
}

func (i *ImageTable) CheckImageTableExists() (TableInfo, bool, error) {
	return CheckImageTableExists(i.DB)
}

func CheckImageTableExists(db *sql.DB) (TableInfo, bool, error) {
	r, err := db.Query("PRAGMA table_list('image');")
	if err != nil {
		return TableInfo{}, false, err
	}

	t := TableInfo{}
	if r.Next() {
		err = r.Scan(&t.Schema, &t.Name, &t.ObjectType, &t.Ncol, &t.WR, &t.Strict)
		if err != nil {
			return TableInfo{}, false, err
		}
	}

	return t, t.Name == "image", nil
}

func (i *ImageTable) CreateImageTable() error {
	_, err := i.DB.Exec(`CREATE TABLE image (
	    id TEXT PRIMARY KEY,
	    mime_type TEXT NOT NULL,
		width INT NOT NULL,
		height INT NOT NULL,
		thumbhash TEXT,
		lat REAL,
		long REAL,
		locality STRING,
		country STRING,
	    created_at DATETIME,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) WITHOUT ROWID;`)
	return err
}

func (i *ImageTable) Save(img Image) error {
	_, err := i.DB.Exec(`
		INSERT INTO image
		(id, mime_type, width, height, thumbhash, lat, long, locality, country, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT DO UPDATE SET country=excluded.country,locality=excluded.locality;`,
		img.ID,
		img.MimeType,
		img.Width,
		img.Height,
		img.ThumbHash,
		img.Lat,
		img.Long,
		img.Locality,
		img.Country,
		img.CreatedAt,
	)

	sqlErr, ok := err.(sqlite.Error)
	if !ok {
		return err
	}

	if sqlErr.ExtendedCode == sqlite.ErrConstraintPrimaryKey {
		return DuplicateImage
	}

	return err
}

func (i *ImageTable) GetByID(id string) (Image, error) {
	row := i.DB.QueryRow("SELECT DISTINCT * FROM image WHERE id = (?);", id)
	if err := row.Err(); err != nil {
		return Image{}, err
	}

	return i.scanImageRow(row)
}

type Order string

const ASC = Order("ASC")
const DESC = Order("DESC")

type GetListOptsFn func(*GetListOpts) error

type GetListOpts struct {
	Order        Order    `json:"order"`
	Countries    []string `json:"countries"`
	Page         int      `json:"page"`
	ExclStartKey string   `json:"exclStartKey"`
	Limit        int      `json:"limit"`
}

type ImageList struct {
	Images []Image
	Cursor *Cursor
}

var DefaultOpts = GetListOpts{
	Order:        ASC,
	Countries:    []string{},
	Page:         0,
	ExclStartKey: "",
	Limit:        5,
}

func (i *ImageTable) GetList(opts ...GetListOptsFn) (ImageList, error) {
	opt := DefaultOpts

	for _, option := range opts {
		err := option(&opt)
		if err != nil {
			return ImageList{}, err
		}
	}

	args := []any{}
	sb := strings.Builder{}
	sb.WriteString("SELECT * FROM image")

	if len(opt.ExclStartKey) > 0 || len(opt.Countries) > 0 {
		sb.WriteString(" WHERE ")

		if len(opt.ExclStartKey) > 0 {
			sb.WriteString("created_at")

			if opt.Order == ASC {
				sb.WriteString(" > ")
			} else {
				sb.WriteString(" < ")
			}

			sb.WriteString("( SELECT created_at FROM image WHERE id = (?) )")
			args = append(args, opt.ExclStartKey)
		}

		if len(opt.Countries) > 0 && len(opt.ExclStartKey) > 0 {
			sb.WriteString(" AND ")

		}

		if len(opt.Countries) > 0 {
			sb.WriteString("country IN (")

			for i := range opt.Countries {
				sb.WriteString("?")

				if i == len(opt.Countries)-1 {
					sb.WriteString(")")
				} else {
					sb.WriteString(",")
				}

				args = append(args, opt.Countries[i])
			}
		}
	}

	sb.WriteString(" ORDER BY created_at")

	if opt.Order == ASC {
		sb.WriteString(" ASC")
	} else {
		sb.WriteString(" DESC")
	}

	if opt.Page > 0 && len(opt.ExclStartKey) == 0 {
		pageSize := 5
		sb.WriteString(" LIMIT (?) OFFSET (?)")
		args = append(args, pageSize)
		args = append(args, pageSize*(opt.Page-1))
	} else {
		sb.WriteString(" LIMIT (?)")
		args = append(args, opt.Limit)
	}

	sb.WriteString(";")

	q := sb.String()

	rows, err := i.DB.Query(q, args...)
	if err != nil {
		return ImageList{}, fmt.Errorf("could not get image rows: %w", err)
	}
	defer rows.Close()

	imgs := []Image{}
	for rows.Next() {
		img, err := i.scanImageRow(rows)
		if err != nil {
			return ImageList{}, err
		}
		imgs = append(imgs, img)
	}

	if len(imgs) > 0 {
		opt.ExclStartKey = imgs[len(imgs)-1].ID
	}

	cursor, err := NewCursor(opt)
	if err != nil {
		return ImageList{}, fmt.Errorf("could not create cursor on GetList: %w", err)
	}

	return ImageList{
		Images: imgs,
		Cursor: cursor,
	}, nil
}

func WithOpts(opts GetListOpts) GetListOptsFn {
	return func(glo *GetListOpts) error {
		*glo = opts
		return nil
	}
}

func WithOrder(o Order) GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Order = o
		return nil
	}
}

func WithDescOrder() GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Order = DESC
		return nil
	}
}

func WithAscOrder() GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Order = ASC
		return nil
	}
}

func WithCountries(countries ...string) GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Countries = countries
		return nil
	}
}

func WithLimit(limit int) GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Limit = limit
		return nil
	}
}

func WithCursor(cursor *Cursor) GetListOptsFn {
	return func(glo *GetListOpts) error {
		*glo = cursor.Opts()
		return nil
	}

}

func WithCursorStr(cursor string) GetListOptsFn {
	return func(glo *GetListOpts) error {
		c, err := ParseCursor(cursor)
		if err != nil {
			return err
		}
		*glo = c.Opts()
		return nil
	}
}

func WithPage(page int) GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.Page = page
		return nil
	}
}

func WithExclStartKey(startKey string) GetListOptsFn {
	return func(glo *GetListOpts) error {
		glo.ExclStartKey = startKey
		return nil
	}
}

type scanner interface {
	Scan(a ...any) error
}

func (i *ImageTable) scanImageRow(s scanner) (Image, error) {
	img := Image{}
	err := s.Scan(
		&img.ID,
		&img.MimeType,
		&img.Width,
		&img.Height,
		&img.ThumbHash,
		&img.Lat,
		&img.Long,
		&img.Locality,
		&img.Country,
		&img.CreatedAt,
		&img.UploadedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Image{}, NotFound

		}
		return Image{}, fmt.Errorf("could not scan image row: %w", err)
	}
	return img, nil
}

func (i *ImageTable) Delete(id string) error {
	_, err := i.DB.Exec("DELETE FROM image WHERE id = (?)", id)
	if err != nil {
		return fmt.Errorf("could not remove image %s : %w", id, err)
	}
	return nil
}

type Locality struct {
	Country    string
	Localities []string
}

func (i *ImageTable) GetLocalities() ([]Locality, error) {
	res, err := i.DB.Query("SELECT country, locality FROM image WHERE country IS NOT \"\";")
	if err != nil {
		return nil, fmt.Errorf("could not get localities: %w", err)
	}

	var countryLocality = make(map[string][]string)
	for res.Next() {
		country, locality := "", ""
		err = res.Scan(&country, &locality)
		if err != nil {
			return nil, fmt.Errorf("could not scan country and locality: %w", err)
		}

		_, ok := countryLocality[country]
		if !ok {
			countryLocality[country] = []string{locality}
		} else {
			countryLocality[country] = append(countryLocality[country], locality)
		}
	}

	localities := []Locality{}
	for k, v := range countryLocality {
		localities = append(localities, Locality{
			Country:    k,
			Localities: v,
		})
	}

	return localities, nil
}

func (i *ImageTable) Close() error {
	return i.DB.Close()
}

type ImageTable struct {
	DB *sql.DB
}

// TableInfo is the information provided by SQLite's table_list pragma
// https://www.sqlite.org/pragma.html#pragma_table_list
type TableInfo struct {
	Schema     string
	Name       string
	ObjectType string
	Ncol       int
	WR         bool
	Strict     bool
}

type Image struct {
	ID         string    `json:"id"`
	MimeType   string    `json:"mimeType"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	ThumbHash  string    `json:"thumbhash"`
	CreatedAt  time.Time `json:"createdAt"`
	UploadedAt time.Time `json:"uploadedAt"`
	Lat        float64   `json:"lat"`
	Long       float64   `json:"long"`
	Locality   string    `json:"locality"`
	Country    string    `json:"country"`
}
