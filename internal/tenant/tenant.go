package tenant

import (
	"context"
	"fmt"

	"github.com/grevus/mcp-jira/internal/knowledge"
	"github.com/grevus/mcp-jira/internal/tracker"
)

// Config — конфигурация одного тенанта из keys.yaml.
type Config struct {
	APIKey        string
	Name          string
	TrackerType   string
	TrackerConfig map[string]string
	ProjectKeys   []string
}

// Tenant — runtime-состояние клиента.
type Tenant struct {
	Config    Config
	Provider  tracker.Provider
	Knowledge knowledge.Store
	Retriever KnowledgeRetriever
}

// KnowledgeRetriever — узкий интерфейс для retriever (чтобы не тащить конкретный тип).
type KnowledgeRetriever interface {
	Search(ctx context.Context, projectKey, query string, topK int) ([]knowledge.Hit, error)
}

// Registry хранит тенантов по имени ключа.
type Registry struct {
	tenants map[string]*Tenant
}

// NewRegistry создаёт пустой Registry.
func NewRegistry() *Registry {
	return &Registry{tenants: make(map[string]*Tenant)}
}

// Register добавляет тенанта в реестр.
func (r *Registry) Register(name string, t *Tenant) {
	r.tenants[name] = t
}

// Resolve возвращает тенанта по имени ключа.
func (r *Registry) Resolve(keyName string) (*Tenant, error) {
	t, ok := r.tenants[keyName]
	if !ok {
		return nil, fmt.Errorf("tenant: unknown key %q", keyName)
	}
	return t, nil
}
