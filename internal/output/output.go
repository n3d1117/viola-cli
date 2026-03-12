package output

import (
	"encoding/json"
	"fmt"
	"io"
)

func JSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func Errorf(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "error: "+format+"\n", args...)
}
