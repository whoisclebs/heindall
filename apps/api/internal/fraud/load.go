package fraud

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func LoadReferences(path string) ([]Reference, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}

	var refs []Reference
	if err := json.NewDecoder(r).Decode(&refs); err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("reference dataset is empty")
	}
	return refs, nil
}
