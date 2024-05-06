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
		VALUES (?,?,?,?,?,?,?,?,?,?);`,
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

type OrderDirection string

const ASC = OrderDirection("ASC")
const DESC = OrderDirection("DESC")

type GetOpts struct {
	OrderDirection OrderDirection
	Countries      []string
	Page           int
}

func (i *ImageTable) Get(opts ...GetOpts) ([]Image, error) {
	direction := ASC
	args := []any{}
	page := 0

	if len(opts) > 0 {
		// ignore other opts
		opt := opts[0]

		page = opt.Page

		if len(opt.OrderDirection) != 0 {
			direction = opt.OrderDirection
		}

		if len(opt.Countries) > 0 {
			for i := range opt.Countries {
				args = append(args, opt.Countries[i])
			}
		}
	}

	sb := strings.Builder{}
	sb.WriteString("SELECT * FROM image")
	if len(args) > 0 {
		sb.WriteString(" WHERE")

		for i := range args {
			sb.WriteString(" country = (?)")

			if i < len(args)-1 {
				sb.WriteString(" OR ")
			}
		}
	}

	sb.WriteString(" ORDER BY created_at")

	if direction == ASC {
		sb.WriteString(" ASC")
	} else {
		sb.WriteString(" DESC")
	}

	if page > 0 {
		pageSize := 5
		sb.WriteString(" LIMIT (?) OFFSET (?)")
		args = append(args, pageSize)
		args = append(args, pageSize*(page-1))
	}

	sb.WriteString(";")

	q := sb.String()

	rows, err := i.DB.Query(q, args...)
	if err != nil {
		return []Image{}, fmt.Errorf("could not get image rows: %w", err)
	}
	defer rows.Close()

	imgs := []Image{}
	for rows.Next() {
		img, err := i.scanImageRow(rows)
		if err != nil {
			return []Image{}, err
		}
		imgs = append(imgs, img)
	}
	return imgs, nil
}

func (i *ImageTable) getASC() (*sql.Rows, error) {
	return i.DB.Query("SELECT * FROM image ORDER BY created_at ASC;")
}

func (i *ImageTable) getDESC() (*sql.Rows, error) {
	return i.DB.Query("SELECT * FROM image ORDER BY created_at DESC;")
}

// GetPrev returns the Image previous to the one pointed to by id
// when ordered by Created time
func (i ImageTable) GetPrev(id string) (Image, error) {
	row := i.DB.QueryRow(`SELECT * FROM image
		WHERE created_at < (
			SELECT created_at FROM image WHERE id = (?)
		) ORDER BY created_at DESC LIMIT 1;`, id)

	if err := row.Err(); err != nil {
		return Image{}, fmt.Errorf("could not get previous row from %s: %w", id, err)
	}

	return i.scanImageRow(row)
}

func (i *ImageTable) GetNext(id string) (Image, error) {
	row := i.DB.QueryRow(`SELECT * FROM image WHERE created_at > (
    SELECT created_at FROM image WHERE id = (?)
) ORDER BY created_at ASC LIMIT 1;`, id)

	if err := row.Err(); err != nil {
		return Image{}, fmt.Errorf("could not get previous row from %s: %w", id, err)
	}

	return i.scanImageRow(row)
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
	ID         string
	MimeType   string
	Width      int
	Height     int
	ThumbHash  string
	CreatedAt  time.Time
	UploadedAt time.Time
	Lat        float64
	Long       float64
	Locality   string
	Country    string
}
