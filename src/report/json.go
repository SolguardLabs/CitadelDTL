package report

import (
	"encoding/json"
	"io"
)

func WriteJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
