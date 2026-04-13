package tenant

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type keyEntry struct {
	Key           string            `yaml:"key"`
	Name          string            `yaml:"name"`
	TrackerType   string            `yaml:"tracker"`
	TrackerConfig map[string]string `yaml:"tracker_config"`
	Projects      []string          `yaml:"projects"`
}

type keysFile struct {
	Keys []keyEntry `yaml:"keys"`
}

// LoadTenantsFromFile reads extended keys.yaml and returns tenant configs.
func LoadTenantsFromFile(path string) ([]Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tenant: read file: %w", err)
	}

	var f keysFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("tenant: parse file: %w", err)
	}

	if len(f.Keys) == 0 {
		return nil, fmt.Errorf("tenant: no keys in %s", path)
	}

	configs := make([]Config, 0, len(f.Keys))
	for i, k := range f.Keys {
		if k.Key == "" {
			return nil, fmt.Errorf("tenant: key #%d has empty value", i+1)
		}
		if k.Name == "" {
			k.Name = fmt.Sprintf("key-%d", i+1)
		}
		configs = append(configs, Config{
			APIKey:        k.Key,
			Name:          k.Name,
			TrackerType:   k.TrackerType,
			TrackerConfig: k.TrackerConfig,
			ProjectKeys:   k.Projects,
		})
	}

	return configs, nil
}
