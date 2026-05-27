package memind_test

import (
	"testing"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/engine"
	"github.com/openmemind/memind-go/store"
)

// TestExtractAndRetrieve - 完整流程测试：添加消息 → 提交 → 检索 → 验证非空
func TestExtractAndRetrieve(t *testing.T) {
	mem := engine.Builder().
		Store(store.NewInMemoryStore()).
		Build()
	defer mem.Close()

	memID := memind.MemoryId{UserID: "test-user"}

	msg := memind.Message{
		Role: memind.RoleUser,
		Content: []memind.ContentBlock{
			{Type: "text", Text: "My name is Bob. I love hiking and photography."},
		},
	}
	_, err := mem.AddMessage(memID, msg, memind.DefaultExtractionConfig())
	if err != nil {
		t.Fatal(err)
	}

	result, err := mem.Commit(memID, memind.DefaultExtractionConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != memind.ExtractionSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if len(result.Items.NewItems) == 0 {
		t.Fatal("expected at least one memory item")
	}

	retResult, err := mem.Retrieve(memind.RetrievalRequest{
		MemoryID: memID,
		Query:    "Tell me about Bob",
	})
	if err != nil {
		t.Fatal(err)
	}
	if retResult.IsEmpty() {
		t.Fatal("expected non-empty retrieval")
	}
}

// TestEmptyQuery - 空查询应返回空结果
func TestEmptyQuery(t *testing.T) {
	mem := engine.Builder().
		Store(store.NewInMemoryStore()).
		Build()
	defer mem.Close()

	result, err := mem.Retrieve(memind.RetrievalRequest{
		MemoryID: memind.MemoryId{UserID: "empty-test"},
		Query:    "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsEmpty() {
		t.Fatal("expected empty result for empty query")
	}
}

// TestDirectExtract - 直接提取测试（不经缓冲区）
func TestDirectExtract(t *testing.T) {
	mem := engine.Builder().
		Store(store.NewInMemoryStore()).
		Build()
	defer mem.Close()

	result, err := mem.Extract(memind.ExtractionRequest{
		MemoryID: memind.MemoryId{UserID: "direct-test"},
		Content:  memind.RawContent{Type: "text", Content: "Alice is a data scientist from San Francisco."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != memind.ExtractionSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
}
