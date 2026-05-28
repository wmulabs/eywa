package trial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func LoadTrialSuite(path string) (entities.TrialSuite, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return entities.TrialSuite{}, fmt.Errorf("read trial suite: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))

	var suite entities.TrialSuite

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &suite); err != nil {
			return entities.TrialSuite{}, fmt.Errorf("parse yaml trial suite: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &suite); err != nil {
			return entities.TrialSuite{}, fmt.Errorf("parse json trial suite: %w", err)
		}
	default:
		return entities.TrialSuite{}, fmt.Errorf("unsupported trial suite format %q; use .yaml, .yml, or .json", ext)
	}

	return suite, nil
}
