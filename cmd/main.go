package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/evanoberholster/imagemeta"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

func main() {
	img, err := os.Open("oteka.jpeg")
	if err != nil {
		panic(err.Error())
	}
	defer img.Close()

	e, err := imagemeta.Decode(img)
	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Println(e)
	img.Close()

	img, err = os.Open("oteka.jpeg")
	if err != nil {
		panic(err.Error())
	}
	defer img.Close()

	exif.RegisterParsers(mknote.All...)

	getExif(img)

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("File Upload Endpoint Hit")

		// Parse our multipart form, 10 << 20 specifies a maximum
		// upload of 10 MB files.
		r.ParseMultipartForm(10 << 20)
		// FormFile returns the first file for the given key `myFile`
		// it also returns the FileHeader so we can get the Filename,
		// the Header and the size of the file
		file, handler, err := r.FormFile("myFile")
		if err != nil {
			fmt.Println("Error Retrieving the File")
			fmt.Println(err)
			return
		}
		defer file.Close()
		fmt.Printf("Uploaded File: %+v\n", handler.Filename)
		fmt.Printf("File Size: %+v\n", handler.Size)
		fmt.Printf("MIME Header: %+v\n", handler.Header)

		tempFile, err := os.CreateTemp("temp-images", "upload-*.jpg")
		if err != nil {
			panic(err.Error())
		}
		defer tempFile.Close()

		bytes, err := io.ReadAll(file)
		if err != nil {
			panic("read all" + err.Error())
		}

		_, err = tempFile.Write(bytes)
		if err != nil {
			panic(err.Error())
		}

		tempFile, err = os.Open(tempFile.Name())
		if err != nil {
			panic(err.Error())
		}

		getExif(tempFile)
	})

	http.ListenAndServe(":8080", nil)

}

func getExif(r io.Reader) {
	x, err := exif.Decode(r)
	if err != nil && exif.IsCriticalError(err) {
		fmt.Println("could not decode exif", err.Error())
		return
	}

	camModel, _ := x.Get(exif.Model) // normally, don't ignore errors!
	fmt.Println(camModel.StringVal())

	focal, _ := x.Get(exif.FocalLength)
	numer, denom, _ := focal.Rat2(0) // retrieve first (only) rat. value
	fmt.Printf("%v/%v", numer, denom)

	// Two convenience functions exist for date/time taken and GPS coords:
	tm, _ := x.DateTime()
	fmt.Println("Taken: ", tm)

	lat, long, _ := x.LatLong()
	fmt.Println("lat, long: ", lat, ", ", long)
}

const jpeg_APP1 = 0xE1

// newAppSec finds marker in r and returns the corresponding application data
// section.
func newAppSec(marker byte, r io.Reader) (*appSec, error) {
	br := bufio.NewReader(r)
	app := &appSec{marker: marker}
	var dataLen int

	// seek to marker
	for dataLen == 0 {
		if _, err := br.ReadBytes(0xFF); err != nil {
			return nil, err
		}
		c, err := br.ReadByte()
		if err != nil {
			return nil, err
		} else if c != marker {
			continue
		}

		dataLenBytes := make([]byte, 2)
		for k, _ := range dataLenBytes {
			c, err := br.ReadByte()
			if err != nil {
				return nil, err
			}
			dataLenBytes[k] = c
		}
		dataLen = int(binary.BigEndian.Uint16(dataLenBytes)) - 2
	}

	// read section data
	nread := 0
	for nread < dataLen {
		s := make([]byte, dataLen-nread)
		n, err := br.Read(s)
		nread += n
		if err != nil && nread < dataLen {
			return nil, err
		}
		app.data = append(app.data, s[:n]...)
	}
	return app, nil
}

type appSec struct {
	marker byte
	data   []byte
}
