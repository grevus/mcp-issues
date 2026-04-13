package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv читает файл path в формате KEY=VALUE и устанавливает переменные окружения,
// которые ещё не заданы. Если файл не найден — молча игнорирует.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// Mode определяет режим запуска сервера.
type Mode string

const (
	ModeStdio Mode = "stdio"
	ModeHTTP  Mode = "http"
	ModeIndex Mode = "index"
)

// Config содержит всю конфигурацию приложения.
type Config struct {
	Mode         Mode
	JiraBaseURL  string
	JiraEmail    string
	JiraAPIToken string
	JiraAuthType string // "basic" (default) | "bearer" (Jira DC PAT)
	DatabaseURL  string
	RAGEmbedder  string // "voyage" | "openai" | "onnx"
	VoyageAPIKey string
	OpenAIAPIKey string
	ONNXModelPath string // путь к директории с model.onnx (только для onnx)
	ONNXLibDir    string // путь к директории с libonnxruntime (только для onnx, опционально)
	MCPAPIKey    string // только для http; legacy single-key
	MCPKeysFile  string // только для http; путь к YAML с ключами (приоритет над MCPAPIKey)
	MCPAddr      string // только для http, default ":8080"
}

// Load читает переменные окружения и возвращает Config для указанного mode.
// Общие обязательные переменные: JIRA_BASE_URL, JIRA_EMAIL, JIRA_API_TOKEN, DATABASE_URL.
func Load(mode Mode) (*Config, error) {
	LoadDotEnv(".env")

	authType := os.Getenv("JIRA_AUTH_TYPE")
	if authType == "" {
		authType = "basic"
	}
	if authType != "basic" && authType != "bearer" {
		return nil, fmt.Errorf("config: JIRA_AUTH_TYPE must be \"basic\" or \"bearer\", got %q", authType)
	}

	required := []struct {
		env string
	}{
		{"JIRA_BASE_URL"},
		{"JIRA_API_TOKEN"},
		{"DATABASE_URL"},
	}
	if authType == "basic" {
		required = append(required, struct{ env string }{"JIRA_EMAIL"})
	}

	values := make(map[string]string, len(required)+1)
	for _, r := range required {
		v := os.Getenv(r.env)
		if v == "" {
			return nil, fmt.Errorf("config: %s is required", r.env)
		}
		values[r.env] = v
	}
	values["JIRA_EMAIL"] = os.Getenv("JIRA_EMAIL")

	embedder := os.Getenv("RAG_EMBEDDER")
	if embedder == "" {
		embedder = "voyage"
	}
	if embedder != "voyage" && embedder != "openai" && embedder != "onnx" {
		return nil, fmt.Errorf("config: RAG_EMBEDDER must be \"voyage\", \"openai\" or \"onnx\", got %q", embedder)
	}

	var voyageKey, openaiKey, onnxModelPath, onnxLibDir string
	switch embedder {
	case "voyage":
		voyageKey = os.Getenv("VOYAGE_API_KEY")
		if voyageKey == "" {
			return nil, fmt.Errorf("config: VOYAGE_API_KEY is required when RAG_EMBEDDER=voyage")
		}
	case "openai":
		openaiKey = os.Getenv("OPENAI_API_KEY")
		if openaiKey == "" {
			return nil, fmt.Errorf("config: OPENAI_API_KEY is required when RAG_EMBEDDER=openai")
		}
	case "onnx":
		onnxModelPath = os.Getenv("ONNX_MODEL_PATH")
		if onnxModelPath == "" {
			return nil, fmt.Errorf("config: ONNX_MODEL_PATH is required when RAG_EMBEDDER=onnx")
		}
		onnxLibDir = os.Getenv("ONNX_LIB_DIR") // опционально
	}

	var mcpAPIKey, mcpKeysFile, mcpAddr string
	if mode == ModeHTTP {
		mcpKeysFile = os.Getenv("MCP_KEYS_FILE")
		mcpAPIKey = os.Getenv("MCP_API_KEY")
		if mcpKeysFile == "" && mcpAPIKey == "" {
			return nil, fmt.Errorf("config: MCP_KEYS_FILE or MCP_API_KEY is required for http mode")
		}
		mcpAddr = os.Getenv("MCP_ADDR")
		if mcpAddr == "" {
			mcpAddr = ":8080"
		}
	}

	return &Config{
		Mode:          mode,
		JiraBaseURL:   values["JIRA_BASE_URL"],
		JiraEmail:     values["JIRA_EMAIL"],
		JiraAPIToken:  values["JIRA_API_TOKEN"],
		JiraAuthType:  authType,
		DatabaseURL:   values["DATABASE_URL"],
		RAGEmbedder:   embedder,
		VoyageAPIKey:  voyageKey,
		OpenAIAPIKey:  openaiKey,
		ONNXModelPath: onnxModelPath,
		ONNXLibDir:    onnxLibDir,
		MCPAPIKey:     mcpAPIKey,
		MCPKeysFile:   mcpKeysFile,
		MCPAddr:       mcpAddr,
	}, nil
}
