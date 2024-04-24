package main

import (
	"context"
	"log"
	"os"

	"github.com/wobwainwwight/sa-photos/geocode"
	"googlemaps.github.io/maps"
)

func main() {
	key := os.Getenv("MAPS_KEY")

	client, err := maps.NewClient(maps.WithAPIKey(key))
	if err != nil {
		panic(err.Error())
	}
	coords := []maps.LatLng{
		{Lat: -50.979125, Lng: -73.190042},
		{Lat: -51.007797, Lng: -73.053200},
		{Lat: -51.027153, Lng: -73.027589},
		{Lat: -41.053536, Lng: -71.517983},
		{Lat: -41.100614, Lng: -71.500922},
		{Lat: -25.164356, Lng: -65.732025},
		{Lat: -25.164144, Lng: -65.732200},
	}

	for _, c := range coords {
		res, err := client.ReverseGeocode(context.Background(), &maps.GeocodingRequest{
			LatLng: &c,
		})

		if err != nil {
			log.Printf("could not geocode from %.6f, %.6f: %s\n", c.Lat, c.Lng, err.Error())
		} else if len(res) == 0 {
			log.Printf("no results for geocode from %.6f, %.6f\n", c.Lat, c.Lng)
		} else {

			loc, country, err := geocode.GetLocalityAndCountry(res)
			if err != nil {
				log.Println(err.Error())
			}
			log.Println(loc, country)
		}

	}

}
