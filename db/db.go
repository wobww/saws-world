package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func NewImageTable(dsn string) (ImageTable, error) {
	if len(strings.TrimSpace(dsn)) == 0 {
		dsn = "file:saws.db"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return ImageTable{}, err
	}

	i := ImageTable{db}
	_, ok, err := i.CheckImageTableExists()
	if !ok {
		err = i.CreateImageTable()
		if err != nil {
			return ImageTable{}, err
		}
	}
	return i, nil
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
		location TEXT,
		width INT NOT NULL,
		height INT NOT NULL,
	    created_at DATETIME,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) WITHOUT ROWID;`)
	return err
}

func (i *ImageTable) Save(img Image) error {
	_, err := i.DB.Exec(`
		INSERT INTO image
		(id, mime_type, location, width, height, created_at)
		VALUES (?,?,?,?,?,?);`,
		img.ID,
		img.MimeType,
		img.Location,
		img.Width,
		img.Height,
		img.CreatedAt,
	)

	return err
}

func (i *ImageTable) GetByID(id string) (Image, error) {
	row := i.DB.QueryRow("SELECT DISTINCT * FROM image WHERE id = (?);", id)
	if err := row.Err(); err != nil {
		return Image{}, err
	}

	return i.scanImageRow(row)
}

func (i *ImageTable) Get() ([]Image, error) {
	rows, err := i.DB.Query("SELECT * FROM image;")
	if err != nil {
		return []Image{}, nil
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

type scanner interface {
	Scan(a ...any) error
}

func (i *ImageTable) scanImageRow(s scanner) (Image, error) {
	img := Image{}
	err := s.Scan(&img.ID, &img.MimeType, &img.Location, &img.Width, &img.Height, &img.CreatedAt, &img.UploadedAt)
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
	Location   string
	Width      int
	Height     int
	CreatedAt  time.Time
	UploadedAt time.Time
}
