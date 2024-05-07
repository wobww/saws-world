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

var InvalidCursor = errors.New("invalid cursor")

func ParseCursor(cursor string) (GetOpts, error) {
	// dec, err := base64.URLEncoding.DecodeString(cursor)
	// if err != nil {
	// 	return GetOpts{}, InvalidCursor
	// }

	// bytes.Split(dec, []byte("#"))

	return GetOpts{}, errors.New("implement me")
}

type GetOpts struct {
	OrderDirection OrderDirection
	Countries      []string
	Page           int
	FromRowID      string
	Limit          int
}

type ImageList struct {
	Images []Image
	Cursor string
}

func (i *ImageTable) GetList(opts ...GetOpts) (ImageList, error) {
	opt := GetOpts{
		OrderDirection: ASC,
		Countries:      []string{},
		Limit:          5,
	}

	if len(opts) > 0 {
		// ignore other opts
		opt = opts[0]
		if opt.Limit == 0 {
			opt.Limit = 5
		}
	}

	args := []any{}
	sb := strings.Builder{}
	sb.WriteString("SELECT * FROM image")

	if len(opt.FromRowID) > 0 || len(opt.Countries) > 0 {
		sb.WriteString(" WHERE ")

		if len(opt.FromRowID) > 0 {
			sb.WriteString("created_at")

			if opt.OrderDirection == ASC {
				sb.WriteString(" > ")
			} else {
				sb.WriteString(" < ")
			}

			sb.WriteString("( SELECT created_at FROM image WHERE id = (?) )")
			args = append(args, opt.FromRowID)
		}

		if len(opt.Countries) > 0 && len(opt.FromRowID) > 0 {
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

	if opt.OrderDirection == ASC {
		sb.WriteString(" ASC")
	} else {
		sb.WriteString(" DESC")
	}

	if opt.Page > 0 && len(opt.FromRowID) == 0 {
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

	fmt.Println(q, args)
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
	return ImageList{Images: imgs}, nil
}

func (i *ImageTable) getASC() (*sql.Rows, error) {
	return i.DB.Query("SELECT * FROM image ORDER BY created_at ASC;")
}

func (i *ImageTable) getDESC() (*sql.Rows, error) {
	return i.DB.Query("SELECT * FROM image ORDER BY created_at DESC;")
}

type GetPrevOpts struct {
	N int
}

// GetPrev returns the Image previous to the one pointed to by id
// when ordered by Created time
func (i ImageTable) GetPrev(id string, opts ...GetPrevOpts) ([]Image, error) {
	limit := 1
	if len(opts) != 0 {
		limit = opts[0].N
	}

	rows, err := i.DB.Query(`SELECT * FROM image
		WHERE created_at < (
			SELECT created_at FROM image WHERE id = (?)
		) ORDER BY created_at DESC LIMIT (?);`, id, limit)

	if err != nil {
		return []Image{}, fmt.Errorf("could not get previous rows from %s: %w", id, err)
	}

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

type GetNextOpts struct {
	N int
}

func (i *ImageTable) GetNext(id string, opts ...GetNextOpts) ([]Image, error) {
	limit := 1
	if len(opts) != 0 {
		limit = opts[0].N
	}

	rows, err := i.DB.Query(`SELECT * FROM image WHERE created_at > (
    SELECT created_at FROM image WHERE id = (?)
) ORDER BY created_at ASC LIMIT (?);`, id, limit)

	if err != nil {
		return nil, fmt.Errorf("could not get next rows from %s: %w", id, err)
	}

	imgs := []Image{}
	for rows.Next() {
		img, err := i.scanImageRow(rows)
		if err != nil {
			return nil, fmt.Errorf("could not get next rows from %s: %w", id, err)
		}
		imgs = append(imgs, img)
	}

	return imgs, nil
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
