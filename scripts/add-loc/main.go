package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: add-loc <img>")
		os.Exit(1)
		return
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return
	}

	h := sha256.New()
	_, err = file.WriteTo(h)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return
	}

	id := fmt.Sprintf("%x", h.Sum(nil))[:12]

	type loc struct {
		Locality string `json:"locality"`
		Country  string `json:"country"`
	}

	b, err := json.Marshal(loc{
		Locality: "Santiago",
		Country:  "Chile",
	})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return
	}

	body := bytes.NewBuffer(b)

	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("https://saws.world/images/%s", id), body)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return
	}

	req.Header.Add("Authorization", os.Getenv("SAWS_AUTH"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return
	}

	buf := bytes.Buffer{}

	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(resp.StatusCode, buf.String())
}
