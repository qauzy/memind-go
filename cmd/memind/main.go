package main

import (
	"flag"
	"github.com/openmemind/memind-go/llm"
	"log"
	"os"
	"time"

	"github.com/openmemind/memind-go/engine"
	"github.com/openmemind/memind-go/server"
	sqlstore "github.com/openmemind/memind-go/store/sql"
	"github.com/openmemind/memind-go/vector"
)

func main() {
	addr := flag.String("addr", ":8018", "server listen address")
	dsn := flag.String("dsn", "parallels:rank%Io1@tcp(192.168.199.97:3306)/memind?charset=utf8mb4&parseTime=True&loc=Local", "MySQL DSN")
	apiKey := flag.String("api-key", "sk-vqlsPCwiB4zKNe6CkWqIiM6yl3BnEFiPe79sdeG2eFbjMqZ5FH6CT43Q4zSD9ipw", "OpenAI API key")
	baseURL := flag.String("base-url", "https://opencode.ai/zen/v1", "OpenAI compatible API base URL")
	model := flag.String("model", "deepseek-v4-flash-free", "LLM model name")
	//apiKey := flag.String("api-key", "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", "OpenAI API key")
	//baseURL := flag.String("base-url", "https://integrate.api.nvidia.com/v1", "OpenAI compatible API base URL")
	//model := flag.String("model", "deepseek-ai/deepseek-v4-flash", "LLM model name")
	embedModel := flag.String("embed-model", "embo-01", "Embedding model name (e.g. text-embedding-3-small, nvidia/llama-3.2-nv-embedqa-1b-v2)")
	embedBaseURL := flag.String("embed-base-url", "https://api.minimaxi.com/v1", "Embedding API base URL (default: same as --base-url)")
	embedKey := flag.String("embed-key", "sk-api-b0Eox4bjZytSm34T8wuPc5xZ5QDwnPadNAC7MC_ugwWcTpfkmjPTId21p-pFWPDnAnpAOULdTDRaDGhe9DpmkimXl6DOkeV_Efl7XMqbFWnfqYbFYEs3UUI", "OpenAI API key")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("memind-go starting with MySQL...")

	if addrEnv := os.Getenv("MEMIND_ADDR"); addrEnv != "" {
		*addr = addrEnv
	}
	if dsnEnv := os.Getenv("MYSQL_DSN"); dsnEnv != "" {
		*dsn = dsnEnv
	}
	if keyEnv := os.Getenv("LLM_API_KEY"); keyEnv != "" {
		*apiKey = keyEnv
	}
	if urlEnv := os.Getenv("LLM_BASE_URL"); urlEnv != "" {
		*baseURL = urlEnv
	}
	if modelEnv := os.Getenv("LLM_MODEL"); modelEnv != "" {
		*model = modelEnv
	}
	if emEnv := os.Getenv("EMBED_MODEL"); emEnv != "" {
		*embedModel = emEnv
	}
	if ebEnv := os.Getenv("EMBED_BASE_URL"); ebEnv != "" {
		*embedBaseURL = ebEnv
	}

	store, err := sqlstore.NewMySQLStore(*dsn)
	if err != nil {
		log.Fatalf("failed to init MySQL store: %v", err)
	}

	builder := engine.Builder().Store(store)

	if *apiKey != "" {
		client := llm.NewOpenAIClient(*apiKey,
			llm.WithBaseURL(*baseURL),
			llm.WithModel(*model),
			llm.WithTimeout(300*time.Second),
		)
		builder.ChatClientForSlot(llm.SlotItemExtraction, client)
		builder.ChatClientForSlot(llm.SlotInsightGenerator, client)
		log.Printf("LLM configured: model=%s baseURL=%s", *model, *baseURL)
	} else {
		log.Printf("LLM not configured (set LLM_API_KEY or --api-key)")
	}

	if *embedKey != "" {
		embedURL := *embedBaseURL
		if embedURL == "" {
			embedURL = *baseURL
		}
		embedModelName := *embedModel
		if embedModelName == "" {
			embedModelName = "text-embedding-3-small"
		}
		embClient := llm.NewOpenAIEmbeddingClient(*embedKey,
			llm.WithEmbeddingBaseURL(embedURL),
			llm.WithEmbeddingModel(embedModelName),
		)
		builder.EmbeddingClient(embClient)
		log.Printf("embedding configured: model=%s baseURL=%s", embedModelName, embedURL)
	}

	_ = vector.NewInMemoryVectorStore // ensure import
	memory := builder.Build()

	srv := server.New(memory, *addr)
	log.Printf("listening on %s", *addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
