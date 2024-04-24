package geocode_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/geocode"
	"googlemaps.github.io/maps"
)

func TestGetLocalityAndCountry(t *testing.T) {

	t.Run("should return lowest admin level if locality is not there", func(t *testing.T) {
		noLocality := loadTestData(t, "testdata/no-locality.json")

		locality, country, err := geocode.GetLocalityAndCountry(noLocality)
		require.NoError(t, err)

		assert.Equal(t, "Torres de Paine", locality)
		assert.Equal(t, "Chile", country)
	})
}

func loadTestData(t *testing.T, path string) []maps.GeocodingResult {
	f, err := os.Open(path)
	require.NoError(t, err)

	dec := json.NewDecoder(f)

	res := []maps.GeocodingResult{}
	err = dec.Decode(&res)
	require.NoError(t, err)
	return res
}
