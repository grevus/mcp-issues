package auth

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Key представляет один API-ключ с именем владельца.
type Key struct {
	Value string `yaml:"key"`
	Name  string `yaml:"name"`
}

type keysFile struct {
	Keys []Key `yaml:"keys"`
}

// LoadKeys читает YAML-файл с API-ключами.
// Формат:
//
//	keys:
//	  - key: "sk-test-abc123"
//	    name: "Alice"
func LoadKeys(path string) ([]Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("auth: read keys file: %w", err)
	}

	var f keysFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("auth: parse keys file: %w", err)
	}

	if len(f.Keys) == 0 {
		return nil, fmt.Errorf("auth: keys file %s contains no keys", path)
	}

	for i, k := range f.Keys {
		if k.Value == "" {
			return nil, fmt.Errorf("auth: key #%d has empty value", i+1)
		}
		if k.Name == "" {
			f.Keys[i].Name = fmt.Sprintf("key-%d", i+1)
		}
	}

	return f.Keys, nil
}
