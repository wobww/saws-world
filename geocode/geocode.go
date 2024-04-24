package geocode

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"googlemaps.github.io/maps"
)

// GetLocalityAndCountry returns the country and closest locality from
// a Google Maps Geocoding response.
//
// We want the country and locality, however if the locality is not there,
// we settle for the highest administrative level that we can find as this will be
// closest to the locality
func GetLocalityAndCountry(reses []maps.GeocodingResult) (string, string, error) {
	locality, country := "", ""
	localityFound, countryFound := false, false
	settledAdminLevel := 0

	for _, res := range reses {
		for _, addr := range res.AddressComponents {
			if localityFound && countryFound {
				break
			}

			if slices.Contains(addr.Types, "country") {
				country = addr.LongName
				countryFound = true
				continue
			}

			if slices.Contains(addr.Types, "locality") {
				locality = addr.LongName
				localityFound = true
				continue
			}

			if !localityFound {
				for _, t := range addr.Types {
					if !strings.HasPrefix(t, "administrative_area_level_") {
						continue
					}

					// there are 1-7 admin levels, this will support up to 9 admin levels
					r, _ := utf8.DecodeLastRuneInString(t)
					adminLevel := int(r - '0')
					if adminLevel <= 0 || adminLevel > 9 {
						continue
					}

					if adminLevel > settledAdminLevel {
						locality = addr.LongName
						settledAdminLevel = adminLevel
					}
				}
			}
		}

	}

	var err error
	if len(locality) == 0 && len(country) == 0 {
		err = errors.New("could not get locality or country from google geocode")
	} else if len(locality) == 0 {
		err = fmt.Errorf("could get country (%s) but not locality from google geocode", country)
	} else if len(country) == 0 {
		err = fmt.Errorf("could get locality (%s) but not country from google geocode", locality)
	}

	return locality, country, err
}
