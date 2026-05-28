package sqlstore

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dialect - 数据库方言类型
type dialect int

const (
	dialectSQLite dialect = iota
	dialectMySQL
)

// autoIncrement - 返回对应方言的自增关键字
func (d dialect) autoIncrement() string {
	if d == dialectMySQL {
		return "AUTO_INCREMENT"
	}
	return "AUTOINCREMENT"
}

// timestampType - 返回对应方言的时间戳类型
func (d dialect) timestampType() string {
	if d == dialectMySQL {
		return "DATETIME(3)"
	}
	return "TIMESTAMP"
}

// booleanType - 返回对应方言的布尔类型
func (d dialect) booleanType() string {
	if d == dialectMySQL {
		return "TINYINT(1)"
	}
	return "INTEGER"
}

// stringType - 返回对应方言的字符串主键类型（MySQL 禁止 TEXT 作主键）
func (d dialect) stringType() string {
	if d == dialectMySQL {
		return "VARCHAR(255)"
	}
	return "TEXT"
}

// upsertSuffix - 返回方言对应的 UPSERT 子句
// MySQL:  ON DUPLICATE KEY UPDATE col=VALUES(col)
// SQLite: ON CONFLICT(pk) DO UPDATE SET col=excluded.col
func (d dialect) upsertSuffix(pk string, cols ...string) string {
	var setPairs []string
	for _, c := range cols {
		if d == dialectMySQL {
			setPairs = append(setPairs, fmt.Sprintf("%s=VALUES(%s)", c, c))
		} else {
			setPairs = append(setPairs, fmt.Sprintf("%s=excluded.%s", c, c))
		}
	}
	if d == dialectMySQL {
		return "ON DUPLICATE KEY UPDATE " + strings.Join(setPairs, ", ")
	}
	return fmt.Sprintf("ON CONFLICT(%s) DO UPDATE SET %s", pk, strings.Join(setPairs, ", "))
}

// createTableSQL - 生成建表 DDL 语句列表，适配当前方言
func (d dialect) createTableSQL() []string {
	ai := d.autoIncrement()
	ts := d.timestampType()

	return []string{
		// memory_items
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS memory_items (
			id INTEGER PRIMARY KEY %s,
			memory_id TEXT NOT NULL,
			content TEXT NOT NULL,
			scope TEXT NOT NULL,
			category TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			source_client TEXT NOT NULL DEFAULT '',
			vector_id TEXT,
			raw_data_id TEXT,
			content_hash TEXT NOT NULL DEFAULT '',
			occurred_at %s,
			occurred_start %s,
			occurred_end %s,
			time_granularity TEXT,
			observed_at %s,
			metadata TEXT,
			created_at %s NOT NULL,
			type TEXT NOT NULL DEFAULT 'FACT'
		)`, ai, ts, ts, ts, ts, ts),

		`CREATE INDEX IF NOT EXISTS idx_items_memory_id ON memory_items(memory_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_hash ON memory_items(content_hash)`,

		// memory_raw_data
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS memory_raw_data (
			id %s PRIMARY KEY,
			memory_id TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			source_client TEXT NOT NULL DEFAULT '',
			content_id TEXT NOT NULL DEFAULT '',
			caption TEXT,
			caption_vector_id TEXT,
			metadata TEXT,
			resource_id TEXT,
			mime_type TEXT,
			created_at %s NOT NULL,
			start_time %s,
			end_time %s
		)`, d.stringType(), ts, ts, ts),

		`CREATE INDEX IF NOT EXISTS idx_raw_data_memory_id ON memory_raw_data(memory_id)`,
		`CREATE INDEX IF NOT EXISTS idx_raw_data_content_id ON memory_raw_data(content_id)`,

		// memory_insights
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS memory_insights (
			id INTEGER PRIMARY KEY %s,
			memory_id TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			scope TEXT NOT NULL DEFAULT 'USER',
			name TEXT NOT NULL DEFAULT '',
			categories TEXT,
			group_name TEXT,
			last_reasoned_at %s,
			summary_embedding TEXT,
			created_at %s NOT NULL,
			updated_at %s NOT NULL,
			tier TEXT NOT NULL DEFAULT 'LEAF',
			parent_insight_id INTEGER,
			child_insight_ids TEXT,
			version INTEGER NOT NULL DEFAULT 1,
			points TEXT
		)`, ai, ts, ts, ts),

		`CREATE INDEX IF NOT EXISTS idx_insights_memory_id ON memory_insights(memory_id)`,
		`CREATE INDEX IF NOT EXISTS idx_insights_type ON memory_insights(type)`,
		`CREATE INDEX IF NOT EXISTS idx_insights_tier ON memory_insights(tier)`,

		// insight_types
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS memory_insight_types (
			id INTEGER PRIMARY KEY %s,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			description_vector_id TEXT,
			categories TEXT,
			target_tokens INTEGER NOT NULL DEFAULT 300,
			analysis_mode TEXT NOT NULL DEFAULT 'BRANCH',
			last_updated_at %s,
			created_at %s NOT NULL,
			updated_at %s NOT NULL,
			scope TEXT NOT NULL DEFAULT 'USER'
		)`, ai, ts, ts, ts),
	}
}

// marshalJSON - 将任意值序列化为 JSON 字符串
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalSlice - 从 JSON 字符串反序列化为切片
func unmarshalSlice[T any](s string) (out []T) {
	if s == "" || s == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// unmarshalMap - 从 JSON 字符串反序列化为 map
func unmarshalMap(s string) map[string]any {
	if s == "" || s == "null" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// unmarshalStringMap - 从 JSON 字符串反序列化为 string map
func unmarshalStringMap(s string) map[string]string {
	if s == "" || s == "null" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// unmarshalFloats - 从 JSON 字符串反序列化为 float32 切片
func unmarshalFloats(s string) []float32 {
	if s == "" || s == "null" {
		return nil
	}
	var out []float32
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// unmarshalInt64Slice - 从 JSON 字符串反序列化为 int64 切片
func unmarshalInt64Slice(s string) []int64 {
	if s == "" || s == "null" {
		return nil
	}
	var out []int64
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// unmarshalStringSlice - 从 JSON 字符串反序列化为 string 切片
func unmarshalStringSlice(s string) []string {
	if s == "" || s == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// placeholderSQL - 生成逗号分隔的占位符列表（用于 IN 子句）
func placeholderSQL(count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, count)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// batchPlaceholders - 生成批量插入的占位符，格式为 (?,?,?), (?,?,?), ...
func batchPlaceholders(cols int, rows int) string {
	if rows <= 0 || cols <= 0 {
		return ""
	}
	parts := make([]string, rows)
	for i := range parts {
		parts[i] = "(" + placeholderSQL(cols) + ")"
	}
	return strings.Join(parts, ",")
}
