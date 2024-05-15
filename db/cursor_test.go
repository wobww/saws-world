package db_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
)

func TestNewCursor(t *testing.T) {

	t.Run("should create cursor", func(t *testing.T) {
		cursor, err := db.NewCursor(db.GetListOpts{
			Order:        db.ASC,
			Countries:    []string{"Chile", "Argentina"},
			Page:         2,
			ExclStartKey: "12345",
			Limit:        2,
		})
		require.NoError(t, err)

		assert.Equal(t, "o:ASC|c:Chile,Argentina|p:2|e:12345|l:2", cursor.String())
	})

	t.Run("should make smaller cursor than json", func(t *testing.T) {
		opts := db.GetListOpts{
			Order:        db.ASC,
			Countries:    []string{"Chile", "Argentina"},
			Page:         2,
			ExclStartKey: "12345",
			Limit:        2,
		}

		cursor, err := db.NewCursor(opts)
		require.NoError(t, err)

		b, err := json.Marshal(opts)
		require.NoError(t, err)
		json := base64.URLEncoding.EncodeToString(b)

		assert.Less(t, len(cursor.EncodedString()), len(json))

		fmt.Println(string(cursor.String()))
		fmt.Println(string(json))
	})

}

func TestParseCursor(t *testing.T) {

	t.Run("blank cursors should fallback to default opts", func(t *testing.T) {
		cursor, err := db.ParseCursor("")
		require.NoError(t, err)

		assert.Equal(t, db.DefaultOpts, cursor.Opts())
		assert.Equal(t, "o:ASC|l:5", cursor.String())
	})

	t.Run("should parse cursor", func(t *testing.T) {
		opts := db.GetListOpts{
			Order:        db.ASC,
			Countries:    []string{"Chile", "Argentina"},
			Page:         2,
			ExclStartKey: "12345",
			Limit:        2,
		}

		cursor, err := db.NewCursor(opts)
		require.NoError(t, err)

		cursor.Debug = true
		err = cursor.Parse(cursor.EncodedString())
		require.NoError(t, err)

		assert.Equal(t, opts, cursor.Opts())
	})

	t.Run("should handle decoded and encoded strings", func(t *testing.T) {
		opts := db.GetListOpts{
			Order:        db.ASC,
			Countries:    []string{"United States", "Chile", "Argentina"},
			Page:         2,
			ExclStartKey: "abc567",
			Limit:        1000,
		}

		cursor, err := db.NewCursor(opts)
		require.NoError(t, err)

		cursorEnc, err := db.ParseCursor(cursor.EncodedString())
		require.NoError(t, err)

		exp := "o:ASC|c:United States,Chile,Argentina|p:2|e:abc567|l:1000"
		assert.Equal(t, opts, cursorEnc.Opts())
		assert.Equal(t, exp, cursorEnc.String())

		cursor.Debug = true
		err = cursor.Parse(cursor.String())
		require.NoError(t, err)

		assert.Equal(t, opts, cursor.Opts())
		assert.Equal(t, exp, cursor.String())
	})

	t.Run("should not include empty opts in cursor", func(t *testing.T) {
		opts := db.GetListOpts{}

		c, err := db.NewCursor(opts)
		require.NoError(t, err)

		assert.Equal(t, "", c.String())
	})

	t.Run("should include only opts that are necessary", func(t *testing.T) {
		tests := []struct {
			opts db.GetListOpts
			exp  string
		}{
			{
				opts: db.GetListOpts{Countries: []string{"United States", "Chile", "Argentina", "Bolivia"}},
				exp:  "c:United States,Chile,Argentina,Bolivia",
			},
			{
				opts: db.GetListOpts{Page: 3},
				exp:  "p:3",
			},
			{
				opts: db.GetListOpts{Page: 200, ExclStartKey: "123jkl"},
				exp:  "p:200|e:123jkl",
			},
			{
				opts: db.GetListOpts{Order: db.ASC, ExclStartKey: "abc123"},
				exp:  "o:ASC|e:abc123",
			},
		}

		for _, tt := range tests {
			t.Run(tt.exp, func(t *testing.T) {
				c, err := db.NewCursor(tt.opts)
				require.NoError(t, err)

				assert.Equal(t, tt.exp, c.String())
			})

		}
	})

	t.Run("should be able to handle any ordering", func(t *testing.T) {
		opts := db.GetListOpts{
			Order:        db.ASC,
			Countries:    []string{"United States", "Chile", "Argentina"},
			Page:         2,
			ExclStartKey: "abc567",
			Limit:        1000,
		}

		tests := []string{
			"o:ASC|c:United States,Chile,Argentina|p:2|e:abc567|l:1000",
			"c:United States,Chile,Argentina|p:2|e:abc567|l:1000|o:ASC",
			"e:abc567|c:United States,Chile,Argentina|p:2|l:1000|o:ASC",
			"e:abc567|p:2|c:United States,Chile,Argentina|l:1000|o:ASC",
			"c:United States,Chile,Argentina|e:abc567|p:2|o:ASC|l:1000",
			"l:1000|e:abc567|p:2|o:ASC|c:United States,Chile,Argentina",
			"l:1000|e:abc567|o:ASC|c:United States,Chile,Argentina|p:2",
		}

		for _, tt := range tests {
			t.Run(tt, func(t *testing.T) {
				cursor, err := db.ParseCursor(tt)
				require.NoError(t, err)

				assert.Equal(t, opts, cursor.Opts())
				assert.Equal(t, tt, cursor.String())
			})
		}
	})

}
