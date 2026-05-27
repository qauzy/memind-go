package sqlstore

import (
	"os"
	"testing"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/engine"
)

// TestSQLiteStore_ExtractAndRetrieve - SQLite 完整流程测试：添加消息 → 提交 → 检索
func TestSQLiteStore_ExtractAndRetrieve(t *testing.T) {
	dbPath := "/tmp/memind-test-" + t.Name() + ".db"
	defer os.Remove(dbPath)

	sqlStore, err := NewSQLiteStore("file:" + dbPath + "?cache=shared")
	if err != nil {
		t.Fatalf("sqlite new: %v", err)
	}

	mem := engine.Builder().
		Store(sqlStore).
		Build()
	defer mem.Close()

	memID := memind.MemoryId{UserID: "test-user"}

	_, err = mem.AddMessage(memID, memind.Message{
		Role:    memind.RoleUser,
		Content: []memind.ContentBlock{{Type: "text", Text: "My name is Bob."}},
	}, memind.DefaultExtractionConfig())
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
		t.Fatal("expected at least one item")
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

// TestSQLiteStore_ItemPersistence - SQLite 条目持久化测试
func TestSQLiteStore_ItemPersistence(t *testing.T) {
	dbPath := "/tmp/memind-test-" + t.Name() + ".db"
	defer os.Remove(dbPath)

	sqlStore, err := NewSQLiteStore("file:" + dbPath + "?cache=shared")
	if err != nil {
		t.Fatalf("sqlite new: %v", err)
	}

	mem := engine.Builder().
		Store(sqlStore).
		Build()
	defer mem.Close()

	memID := memind.MemoryId{UserID: "persist-test"}

	result, err := mem.Extract(memind.ExtractionRequest{
		MemoryID: memID,
		Content:  memind.RawContent{Type: "text", Content: "Alice is a data scientist."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items.NewItems) == 0 {
		t.Fatal("expected items")
	}

	itemID := result.Items.NewItems[0].ID
	if itemID == 0 {
		t.Fatal("expected non-zero item ID from DB")
	}

	items, err := sqlStore.Items().ListItems(memID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected items in store")
	}

	item, err := sqlStore.Items().GetItem(memID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Content != "Alice is a data scientist." {
		t.Fatalf("wrong content: %s", item.Content)
	}
}

// TestSQLiteStore_RawDataPersistence - SQLite 原始数据持久化测试
func TestSQLiteStore_RawDataPersistence(t *testing.T) {
	dbPath := "/tmp/memind-test-" + t.Name() + ".db"
	defer os.Remove(dbPath)

	sqlStore, err := NewSQLiteStore("file:" + dbPath + "?cache=shared")
	if err != nil {
		t.Fatalf("sqlite new: %v", err)
	}

	rawID := "test-raw-001"
	memID := memind.MemoryId{UserID: "raw-test"}
	rd := &memind.MemoryRawData{
		ID: rawID, MemoryID: memID.Identifier(),
		ContentType: "text", ContentID: "cid-001",
		Caption: "hello world",
	}

	err = sqlStore.RawData().UpsertRawData(memID, []*memind.MemoryRawData{rd})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := sqlStore.RawData().GetRawData(memID, rawID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Caption != "hello world" {
		t.Fatalf("wrong caption: %s", loaded.Caption)
	}

	byContent, err := sqlStore.RawData().GetRawDataByContentID(memID, "cid-001")
	if err != nil {
		t.Fatal(err)
	}
	if byContent.ID != rawID {
		t.Fatalf("wrong id: %s", byContent.ID)
	}

	list, err := sqlStore.RawData().ListRawData(memID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatal("expected 1 raw data")
	}
}
