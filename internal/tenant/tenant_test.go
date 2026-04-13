package tenant

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadTenantsFromFile_Full(t *testing.T) {
	path := writeTemp(t, `
keys:
  - key: secret-key-1
    name: acme
    tracker: jira
    tracker_config:
      base_url: https://acme.atlassian.net
      email: user@acme.com
      token: tok123
    projects:
      - ACME
      - INFRA
`)
	configs, err := LoadTenantsFromFile(path)
	require.NoError(t, err)
	require.Len(t, configs, 1)

	c := configs[0]
	require.Equal(t, "secret-key-1", c.APIKey)
	require.Equal(t, "acme", c.Name)
	require.Equal(t, "jira", c.TrackerType)
	require.Equal(t, "https://acme.atlassian.net", c.TrackerConfig["base_url"])
	require.Equal(t, "user@acme.com", c.TrackerConfig["email"])
	require.Equal(t, "tok123", c.TrackerConfig["token"])
	require.Equal(t, []string{"ACME", "INFRA"}, c.ProjectKeys)
}

func TestLoadTenantsFromFile_Legacy(t *testing.T) {
	// YAML без tracker-полей — обратная совместимость со старым форматом keys.yaml
	path := writeTemp(t, `
keys:
  - key: legacy-key
    name: legacy-tenant
`)
	configs, err := LoadTenantsFromFile(path)
	require.NoError(t, err)
	require.Len(t, configs, 1)

	c := configs[0]
	require.Equal(t, "legacy-key", c.APIKey)
	require.Equal(t, "legacy-tenant", c.Name)
	require.Empty(t, c.TrackerType)
	require.Nil(t, c.TrackerConfig)
	require.Nil(t, c.ProjectKeys)
}

func TestLoadTenantsFromFile_EmptyKey(t *testing.T) {
	path := writeTemp(t, `
keys:
  - key: ""
    name: bad-tenant
`)
	_, err := LoadTenantsFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty value")
}

func TestLoadTenantsFromFile_NoKeys(t *testing.T) {
	path := writeTemp(t, `
keys: []
`)
	_, err := LoadTenantsFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no keys")
}

func TestLoadTenantsFromFile_DefaultName(t *testing.T) {
	path := writeTemp(t, `
keys:
  - key: key-without-name
`)
	configs, err := LoadTenantsFromFile(path)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	require.Equal(t, "key-1", configs[0].Name)
}

func TestRegistry_Resolve(t *testing.T) {
	reg := NewRegistry()

	tenant1 := &Tenant{Config: Config{APIKey: "key-abc", Name: "alpha"}}
	reg.Register("key-abc", tenant1)

	got, err := reg.Resolve("key-abc")
	require.NoError(t, err)
	require.Equal(t, tenant1, got)

	_, err = reg.Resolve("unknown-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown key")
}
