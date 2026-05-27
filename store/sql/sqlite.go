package sqlstore

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NewSQLiteStore - 创建 SQLite 存储实例，自动执行迁移
// dsn 示例: "file:memind.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000"
func NewSQLiteStore(dsn string) (*SQLStore, error) {
	if dsn == "" {
		dsn = "file:memind.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := NewStore(db, "sqlite")
	if err := s.Init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite init: %w", err)
	}
	return s, nil
}
