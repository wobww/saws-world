package router_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/db"
	"github.com/wobwainwwight/sa-photos/db/dbtest"
	"github.com/wobwainwwight/sa-photos/image/imagetest"
	"github.com/wobwainwwight/sa-photos/router"
	"github.com/wobwainwwight/sa-photos/templates"
)

func TestRouter(t *testing.T) {
	clearFailedTests(t)

	table := dbtest.NewTestTable(t)
	defer table.Close()

	imgs := make([]db.Image, 100)
	for i := 0; i < 100; i++ {
		imgs[i] = dbtest.GivenImage(t)
	}

	dbtest.GivenSaved(t, table, dbtest.SpaceByHour(imgs)...)

	tmpl, err := templates.GetTemplates()
	require.NoError(t, err)

	srv := router.NewRouter(router.Services{
		ImageFileStore: imagetest.NewStore(),
		Templates:      tmpl,
		ImageTable:     table.ImageTable,
	}, router.Options{
		IncludeIndexPage: false,
	})

	scenarios := []scenario{
		{
			Method:         http.MethodGet,
			URL:            "/south-america",
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("south-america.html"), router.ImagesPage{
				OrderBy:        "oldest",
				CountryFilters: router.NewCountryFilters(),
				Images: router.ToImageListItems(imgs[:5], false, "", db.MustNewCursor(db.GetListOpts{
					Order:        db.ASC,
					ExclStartKey: imgs[4].ID,
					Limit:        5,
				}).EncodedString()),
				UploadEnabled: false,
			}),
		},
		{
			Name:           "OrderLatest",
			Method:         http.MethodGet,
			URL:            "/south-america?order=latest",
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("south-america.html"), router.ImagesPage{
				OrderBy:        "latest",
				CountryFilters: router.NewCountryFilters(),
				Images: router.ToImageListItems(reverse(imgs[95:]), false, "", db.MustNewCursor(db.GetListOpts{
					Order:        db.DESC,
					ExclStartKey: imgs[95].ID,
					Limit:        5,
				}).EncodedString()),
				UploadEnabled: false,
			}),
		},
		{
			Name:           "List",
			Method:         http.MethodGet,
			URL:            "/south-america/images/list",
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("image-list-items"), router.ImagesPage{
				Images: router.ToImageListItems(imgs[:5], false, "", db.MustNewCursor(db.GetListOpts{
					Order:        db.ASC,
					ExclStartKey: imgs[4].ID,
					Limit:        5,
				}).EncodedString()),
			}),
		},
		{
			Name:   "ListWithCursor",
			Method: http.MethodGet,
			URL: fmt.Sprintf("/south-america/images/list?cursor=%s", db.MustNewCursor(db.GetListOpts{
				ExclStartKey: imgs[50].ID,
				Limit:        10,
				Order:        db.ASC,
			}).EncodedString()),
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("image-list-items"), router.ImagesPage{
				Images: router.ToImageListItems(imgs[51:61], false, "", db.MustNewCursor(db.GetListOpts{
					Order:        db.ASC,
					ExclStartKey: imgs[60].ID,
					Limit:        10,
				}).EncodedString()),
			}),
		},
		{
			Name:   "ReverseListWithCursor",
			Method: http.MethodGet,
			URL: fmt.Sprintf("/south-america/images/list?cursor=%s&pagination=reverse", db.MustNewCursor(db.GetListOpts{
				ExclStartKey: imgs[50].ID,
				Limit:        10,
				Order:        db.ASC,
			}).EncodedString()),
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("image-list-items"), router.ImagesPage{
				Images: router.ToImageListItems(reverse(imgs[51:61]), false, db.MustNewCursor(db.GetListOpts{
					Order:        db.ASC,
					ExclStartKey: imgs[60].ID,
					Limit:        10,
				}).EncodedString(), ""),
			}),
		},
		{
			Name:           "ImagePage",
			Method:         http.MethodGet,
			URL:            fmt.Sprintf("/south-america/images/%s", imgs[10].ID),
			ExpectedStatus: 200,
			ExpectedContent: templateBody(t, tmpl.Lookup("south-america-image.html"), router.ImagePage{
				Title:     "South America " + imgs[10].ID,
				ID:        imgs[10].ID,
				ImageURL:  fmt.Sprintf("/images/%s", imgs[10].ID),
				Width:     imgs[10].Width,
				Height:    imgs[10].Height,
				ThumbHash: imgs[10].ThumbHash,
				PrevURL:   fmt.Sprintf("/south-america/images/%s", imgs[9].ID),
				NextURL:   fmt.Sprintf("/south-america/images/%s", imgs[11].ID),
			}),
		},
	}

	for _, s := range scenarios {
		t.Run(s.TestName(), func(t *testing.T) {
			req, err := http.NewRequest(s.Method, s.URL, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			srv.ServeHTTP(rr, req)

			assert.Equal(t, s.ExpectedStatus, rr.Result().StatusCode)
			res := rr.Body.String()

			assert.Equal(t, s.ExpectedContent, res)
			if s.ExpectedContent != res {
				saveBodyDiff(t, s, res)
			}
		})

	}

}

type scenario struct {
	Name   string
	Method string
	URL    string

	ExpectedStatus  int
	ExpectedContent string
}

func (s scenario) TestName() string {
	if len(s.Name) != 0 {
		return s.Name
	}

	return fmt.Sprintf("%s:%s", s.Method, s.URL)
}

func templateBody(t *testing.T, tmpl *template.Template, data any) string {
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, data)
	require.NoError(t, err, "could not create template body")
	return buf.String()
}

func reverse[S any](s []S) []S {
	arr := make([]S, len(s))
	copy(arr, s)
	slices.Reverse(arr)
	return arr
}

func clearFailedTests(t *testing.T) {
	if _, err := os.Stat("./failed-tests"); os.IsNotExist(err) {
		return
	}

	err := os.RemoveAll("./failed-tests")
	require.NoError(t, err, "error while clearing failed tests")
}

func saveBodyDiff(t *testing.T, s scenario, res string) {
	if _, err := os.Stat("./failed-tests"); os.IsNotExist(err) {
		err = os.MkdirAll("./failed-tests", 0755)
		require.NoError(t, err)
	}

	testID := base64.URLEncoding.EncodeToString([]byte(s.TestName()))[:6]

	t.Errorf("%s body is not as expected, check diff %s", s.TestName(), testID)

	exp, err := os.Create(fmt.Sprintf("./failed-tests/exp-%s.html", testID))
	require.NoError(t, err)
	defer exp.Close()

	_, err = exp.WriteString(s.ExpectedContent)
	require.NoError(t, err)

	act, err := os.Create(fmt.Sprintf("./failed-tests/act-%s.html", testID))
	require.NoError(t, err)
	defer act.Close()

	_, err = act.WriteString(res)
	require.NoError(t, err)
}
