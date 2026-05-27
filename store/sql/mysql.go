package sqlstore

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// NewMySQLStore - 创建 MySQL 存储实例，自动执行迁移
// dsn 示例: "root:password@tcp(127.0.0.1:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
func NewMySQLStore(dsn string) (*SQLStore, error) {
	if dsn == "" {
		dsn = "root:password@tcp(127.0.0.1:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	s := NewStore(db, "mysql")
	if err := s.Init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql init: %w", err)
	}
	return s, nil
}
