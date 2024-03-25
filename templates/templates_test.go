package templates_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wobwainwwight/sa-photos/templates"
)

func TestGetTemplates(t *testing.T) {

	t.Run("should return all templates", func(t *testing.T) {

		tempNames := []string{
			templates.Index,
			templates.SouthAmerica,
		}

		tmps, err := templates.GetTemplates()
		require.NoError(t, err)

		for _, tn := range tempNames {
			t.Run(tn, func(t *testing.T) {
				tm := tmps.Lookup(tn)
				require.NotNil(t, tm)
			})
		}

	})
}
