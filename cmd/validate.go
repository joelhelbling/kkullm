package cmd

import (
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/model"
)

// validateStatus rejects a card status that is not one of model.AllStatuses,
// naming the full valid set so the caller can correct it without guessing.
func validateStatus(s string) error {
	if model.ValidStatuses[s] {
		return nil
	}
	return fmt.Errorf("invalid status %q: must be one of %s", s, strings.Join(model.AllStatuses, ", "))
}

// cardListFormats is the set of accepted --format values for `card list`.
var cardListFormats = []string{"brief", "full"}

// validateFormat rejects a `card list --format` value outside cardListFormats.
func validateFormat(s string) error {
	for _, f := range cardListFormats {
		if s == f {
			return nil
		}
	}
	return fmt.Errorf("invalid --format %q: must be one of %s", s, strings.Join(cardListFormats, ", "))
}
