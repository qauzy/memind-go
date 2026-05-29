package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/store"
)

// SQLStore - 基于 database/sql 的持久化存储，支持 SQLite 和 MySQL
type SQLStore struct {
	db      *sql.DB
	dialect dialect
	rawOps  *SQLRawDataOps
	itemOps *SQLItemOps
	insOps  *SQLInsightOps
}

// NewStore - 创建 SQL 存储实例，根据驱动名称自动检测方言
func NewStore(db *sql.DB, driverName string) *SQLStore {
	d := dialectSQLite
	if strings.Contains(driverName, "mysql") {
		d = dialectMySQL
	}
	s := &SQLStore{db: db, dialect: d}
	s.rawOps = &SQLRawDataOps{db: db, dialect: d}
	s.itemOps = &SQLItemOps{db: db}
	s.insOps = &SQLInsightOps{db: db, dialect: d}
	return s
}

// Init - 执行数据库迁移：创建所有表并插入默认洞察类型
func (s *SQLStore) Init() error {
	for _, ddl := range s.dialect.createTableSQL() {
		if _, err := s.db.Exec(ddl); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	types := s.insOps.defaultInsightTypes()
	if len(types) > 0 {
		s.insOps.UpsertInsightTypes(types)
	}
	return nil
}

// DB - 返回底层 database/sql 连接（用于自定义查询）
func (s *SQLStore) DB() *sql.DB { return s.db }

// RawData - 返回原始数据操作接口
func (s *SQLStore) RawData() store.RawDataOperations { return s.rawOps }

// Items - 返回条目操作接口
func (s *SQLStore) Items() store.ItemOperations { return s.itemOps }

// Insights - 返回洞察操作接口
func (s *SQLStore) Insights() store.InsightOperations { return s.insOps }

// ---------- RawData ----------

// SQLRawDataOps - SQL 版原始数据存储操作
type SQLRawDataOps struct {
	db      *sql.DB
	dialect dialect
}

// scanRawData - 扫描一行数据到 MemoryRawData 结构体
func scanRawData(scanner interface{ Scan(...any) error }) (*memind.MemoryRawData, error) {
	var id, memoryID, contentType, sourceClient, contentID string
	var caption, captionVectorID, metadataStr, resourceID, mimeType sql.NullString
	var createdAt time.Time
	var startTime, endTime sql.NullTime

	err := scanner.Scan(&id, &memoryID, &contentType, &sourceClient, &contentID,
		&caption, &captionVectorID, &metadataStr, &resourceID, &mimeType,
		&createdAt, &startTime, &endTime)
	if err != nil {
		return nil, err
	}
	rd := &memind.MemoryRawData{
		ID: id, MemoryID: memoryID, ContentType: contentType,
		SourceClient: sourceClient, ContentID: contentID,
		Caption: caption.String, CaptionVectorID: captionVectorID.String,
		Metadata:   unmarshalMap(metadataStr.String),
		ResourceID: resourceID.String, MimeType: mimeType.String,
		CreatedAt: createdAt,
	}
	if startTime.Valid {
		rd.StartTime = &startTime.Time
	}
	if endTime.Valid {
		rd.EndTime = &endTime.Time
	}
	return rd, nil
}

// UpsertRawData - 插入或更新原始数据
func (o *SQLRawDataOps) UpsertRawData(memoryID memind.MemoryId, rawData []*memind.MemoryRawData) error {
	for _, rd := range rawData {
		if rd.ID == "" {
			continue
		}
		if rd.CreatedAt.IsZero() {
			rd.CreatedAt = time.Now()
		}
		_, err := o.db.Exec(fmt.Sprintf(`
			INSERT INTO memory_raw_data(id, memory_id, content_type, source_client, content_id,
				caption, caption_vector_id, metadata, resource_id, mime_type,
				created_at, start_time, end_time)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
			%s`, o.dialect.upsertSuffix("id", "caption", "metadata", "caption_vector_id")),
			rd.ID, rd.MemoryID, rd.ContentType, rd.SourceClient, rd.ContentID,
			nullStr(rd.Caption), nullStr(rd.CaptionVectorID),
			nullStr(marshalJSON(rd.Metadata)), nullStr(rd.ResourceID), nullStr(rd.MimeType),
			rd.CreatedAt, nullTime(rd.StartTime), nullTime(rd.EndTime))
		if err != nil {
			return fmt.Errorf("upsert raw_data %s: %w", rd.ID, err)
		}
	}
	return nil
}

// GetRawData - 按 ID 获取原始数据
func (o *SQLRawDataOps) GetRawData(memoryID memind.MemoryId, rawDataID string) (*memind.MemoryRawData, error) {
	row := o.db.QueryRow(`SELECT id, memory_id, content_type, source_client, content_id,
		caption, caption_vector_id, metadata, resource_id, mime_type,
		created_at, start_time, end_time FROM memory_raw_data WHERE id=?`, rawDataID)
	return scanRawData(row)
}

// GetRawDataByContentID - 按 contentID 获取原始数据
func (o *SQLRawDataOps) GetRawDataByContentID(memoryID memind.MemoryId, contentID string) (*memind.MemoryRawData, error) {
	row := o.db.QueryRow(`SELECT id, memory_id, content_type, source_client, content_id,
		caption, caption_vector_id, metadata, resource_id, mime_type,
		created_at, start_time, end_time FROM memory_raw_data
		WHERE memory_id=? AND content_id=? LIMIT 1`, memoryID.Identifier(), contentID)
	return scanRawData(row)
}

// ListRawDataByContentID - 按 contentID 列出原始数据
func (o *SQLRawDataOps) ListRawDataByContentID(memoryID memind.MemoryId, contentID string) ([]*memind.MemoryRawData, error) {
	rows, err := o.db.Query(`SELECT id, memory_id, content_type, source_client, content_id,
		caption, caption_vector_id, metadata, resource_id, mime_type,
		created_at, start_time, end_time FROM memory_raw_data
		WHERE memory_id=? AND content_id=?`, memoryID.Identifier(), contentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryRawData
	for rows.Next() {
		rd, err := scanRawData(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rd)
	}
	return result, nil
}

// ListRawData - 列出指定 memory 的所有原始数据
func (o *SQLRawDataOps) ListRawData(memoryID memind.MemoryId) ([]*memind.MemoryRawData, error) {
	rows, err := o.db.Query(`SELECT id, memory_id, content_type, source_client, content_id,
		caption, caption_vector_id, metadata, resource_id, mime_type,
		created_at, start_time, end_time FROM memory_raw_data WHERE memory_id=?`,
		memoryID.Identifier())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryRawData
	for rows.Next() {
		rd, err := scanRawData(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rd)
	}
	return result, nil
}

// PollRawDataWithoutVector - 查询尚未生成向量的原始数据（用于异步向量生成）
func (o *SQLRawDataOps) PollRawDataWithoutVector(memoryID memind.MemoryId, limit int, minAge time.Duration) ([]*memind.MemoryRawData, error) {
	cutoff := time.Now().Add(-minAge)
	rows, err := o.db.Query(`SELECT id, memory_id, content_type, source_client, content_id,
		caption, caption_vector_id, metadata, resource_id, mime_type,
		created_at, start_time, end_time FROM memory_raw_data
		WHERE memory_id=? AND (caption_vector_id IS NULL OR caption_vector_id='') AND created_at < ? LIMIT ?`,
		memoryID.Identifier(), cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryRawData
	for rows.Next() {
		rd, err := scanRawData(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, rd)
	}
	return result, nil
}

// UpdateRawDataVectorIDs - 批量更新原始数据的向量 ID
func (o *SQLRawDataOps) UpdateRawDataVectorIDs(memoryID memind.MemoryId, vectorIDs map[string]string, metadataPatch map[string]any) error {
	metaJSON := marshalJSON(metadataPatch)
	for id, vecID := range vectorIDs {
		_, err := o.db.Exec(`UPDATE memory_raw_data SET caption_vector_id=?, metadata=? WHERE id=?`,
			vecID, metaJSON, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteRawData - 删除指定原始数据
func (o *SQLRawDataOps) DeleteRawData(memoryID memind.MemoryId, rawDataID string) error {
	_, err := o.db.Exec(`DELETE FROM memory_raw_data WHERE id=? AND memory_id=?`, rawDataID, memoryID.Identifier())
	return err
}

// ---------- Items ----------

// SQLItemOps - SQL 版记忆条目存储操作
type SQLItemOps struct{ db *sql.DB }

// scanItem - 扫描一行数据到 MemoryItem 结构体
func scanItem(scanner interface{ Scan(...any) error }) (*memind.MemoryItem, error) {
	var id int64
	var memoryID, content, scope, category, contentType, sourceClient string
	var vectorID, rawDataID, contentHash, timeGranularity, metadataStr, typ sql.NullString
	var occurredAt, occurredStart, occurredEnd, observedAt sql.NullTime
	var createdAt time.Time

	err := scanner.Scan(&id, &memoryID, &content, &scope, &category,
		&contentType, &sourceClient, &vectorID, &rawDataID, &contentHash,
		&occurredAt, &occurredStart, &occurredEnd, &timeGranularity,
		&observedAt, &metadataStr, &createdAt, &typ)
	if err != nil {
		return nil, err
	}
	item := &memind.MemoryItem{
		ID: id, MemoryID: memoryID, Content: content,
		Scope: memind.MemoryScope(scope), Category: memind.MemoryCategory(category),
		ContentType: contentType, SourceClient: sourceClient,
		VectorID: vectorID.String, RawDataID: rawDataID.String,
		ContentHash: contentHash.String, TimeGranularity: timeGranularity.String,
		Metadata:  unmarshalMap(metadataStr.String),
		CreatedAt: createdAt, Type: memind.MemoryItemType("fact"),
	}
	if typ.Valid {
		item.Type = memind.MemoryItemType(typ.String)
	}
	if occurredAt.Valid {
		item.OccurredAt = &occurredAt.Time
	}
	if occurredStart.Valid {
		item.OccurredStart = &occurredStart.Time
	}
	if occurredEnd.Valid {
		item.OccurredEnd = &occurredEnd.Time
	}
	if observedAt.Valid {
		item.ObservedAt = &observedAt.Time
	}
	return item, nil
}

// UpsertItems - 插入或更新记忆条目
func (o *SQLItemOps) UpsertItems(memoryID memind.MemoryId, items []*memind.MemoryItem) error {
	for _, item := range items {
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}
		var exists bool
		if item.ID > 0 {
			o.db.QueryRow(`SELECT 1 FROM memory_items WHERE id=?`, item.ID).Scan(&exists)
		}
		if exists {
			_, err := o.db.Exec(`UPDATE memory_items SET content=?,scope=?,category=?,vector_id=?,
				raw_data_id=?,content_hash=?,metadata=?,type=? WHERE id=?`,
				item.Content, string(item.Scope), string(item.Category),
				nullStr(item.VectorID), nullStr(item.RawDataID),
				item.ContentHash, marshalJSON(item.Metadata), string(item.Type), item.ID)
			if err != nil {
				return err
			}
		} else {
			res, err := o.db.Exec(`INSERT INTO memory_items(memory_id,content,scope,category,content_type,
				source_client,vector_id,raw_data_id,content_hash,occurred_at,occurred_start,occurred_end,
				time_granularity,observed_at,metadata,created_at,type)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				item.MemoryID, item.Content, string(item.Scope), string(item.Category),
				item.ContentType, item.SourceClient, nullStr(item.VectorID), nullStr(item.RawDataID),
				item.ContentHash, nullTime(item.OccurredAt), nullTime(item.OccurredStart),
				nullTime(item.OccurredEnd), nullStr(item.TimeGranularity),
				nullTime(item.ObservedAt), marshalJSON(item.Metadata), item.CreatedAt, string(item.Type))
			if err != nil {
				return err
			}
			if item.ID == 0 {
				id, _ := res.LastInsertId()
				item.ID = id
			}
		}
	}
	return nil
}

// GetItem - 按 ID 获取记忆条目
func (o *SQLItemOps) GetItem(memoryID memind.MemoryId, itemID int64) (*memind.MemoryItem, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,content,scope,category,content_type,source_client,
		vector_id,raw_data_id,content_hash,occurred_at,occurred_start,occurred_end,
		time_granularity,observed_at,metadata,created_at,type FROM memory_items WHERE id=?`, itemID)
	return scanItem(row)
}

// ListItems - 列出指定 memory 的所有条目
func (o *SQLItemOps) ListItems(memoryID memind.MemoryId) ([]*memind.MemoryItem, error) {
	rows, err := o.db.Query(`SELECT id,memory_id,content,scope,category,content_type,source_client,
		vector_id,raw_data_id,content_hash,occurred_at,occurred_start,occurred_end,
		time_granularity,observed_at,metadata,created_at,type FROM memory_items WHERE memory_id=?`,
		memoryID.Identifier())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// DeleteItems - 批量删除条目
func (o *SQLItemOps) DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(itemIDs))
	args := make([]any, len(itemIDs))
	for i, id := range itemIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(`DELETE FROM memory_items WHERE id IN(%s) AND memory_id=?`,
		strings.Join(placeholders, ","))
	args = append(args, memoryID.Identifier())
	_, err := o.db.Exec(q, args...)
	return err
}

// GetItemByHash - 按内容哈希获取条目（用于去重）
func (o *SQLItemOps) GetItemByHash(memoryID memind.MemoryId, hash string) (*memind.MemoryItem, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,content,scope,category,content_type,source_client,
		vector_id,raw_data_id,content_hash,occurred_at,occurred_start,occurred_end,
		time_granularity,observed_at,metadata,created_at,type FROM memory_items
		WHERE memory_id=? AND content_hash=? LIMIT 1`, memoryID.Identifier(), hash)
	return scanItem(row)
}

// ---------- Insights ----------

// SQLInsightOps - SQL 版洞察存储操作
type SQLInsightOps struct {
	db      *sql.DB
	dialect dialect
}

// scanInsight - 扫描一行数据到 MemoryInsight 结构体
func scanInsight(scanner interface{ Scan(...any) error }) (*memind.MemoryInsight, error) {
	var id int64
	var memoryID, typ, scope, name string
	var categories, groupName, summaryEmbedding, tier, childIDs, pointsStr sql.NullString
	var lastReasonedAt sql.NullTime
	var createdAt, updatedAt time.Time
	var parentInsightID sql.NullInt64
	var version int

	err := scanner.Scan(&id, &memoryID, &typ, &scope, &name,
		&categories, &groupName, &lastReasonedAt, &summaryEmbedding,
		&createdAt, &updatedAt, &tier, &parentInsightID, &childIDs, &version, &pointsStr)
	if err != nil {
		return nil, err
	}

	ins := &memind.MemoryInsight{
		ID: id, MemoryID: memoryID, Type: typ,
		Scope: memind.MemoryScope(scope), Name: name,
		Categories:       unmarshalStringSlice(categories.String),
		Group:            groupName.String,
		SummaryEmbedding: unmarshalFloats(summaryEmbedding.String),
		CreatedAt:        createdAt, UpdatedAt: updatedAt,
		Tier:            memind.InsightTier(tier.String),
		ChildInsightIDs: unmarshalInt64Slice(childIDs.String),
		Version:         version,
		Points:          unmarshalSlice[memind.InsightPoint](pointsStr.String),
	}
	if lastReasonedAt.Valid {
		ins.LastReasonedAt = &lastReasonedAt.Time
	}
	if parentInsightID.Valid {
		ins.ParentInsightID = &parentInsightID.Int64
	}
	return ins, nil
}

// UpsertInsightTypes - 插入或更新洞察类型
func (o *SQLInsightOps) UpsertInsightTypes(types []*memind.MemoryInsightType) error {
	for _, t := range types {
		if t.CreatedAt.IsZero() {
			t.CreatedAt = time.Now()
		}
		t.UpdatedAt = time.Now()
		_, err := o.db.Exec(fmt.Sprintf(`INSERT INTO memory_insight_types(name,description,description_vector_id,
			categories,target_tokens,analysis_mode,last_updated_at,created_at,updated_at,scope)
			VALUES(?,?,?,?,?,?,?,?,?,?)
			%s`, o.dialect.upsertSuffix("name", "description", "categories", "target_tokens", "analysis_mode", "updated_at")),
			t.Name, t.Description, nullStr(t.DescriptionVectorID),
			marshalJSON(t.Categories), t.TargetTokens, string(t.AnalysisMode),
			t.LastUpdatedAt, t.CreatedAt, t.UpdatedAt, string(t.Scope))
		if err != nil {
			return fmt.Errorf("upsert insight type %s: %w", t.Name, err)
		}
	}
	return nil
}

// GetInsightType - 按名称获取洞察类型
func (o *SQLInsightOps) GetInsightType(name string) (*memind.MemoryInsightType, error) {
	row := o.db.QueryRow(`SELECT id,name,description,description_vector_id,categories,target_tokens,
		analysis_mode,last_updated_at,created_at,updated_at,scope FROM memory_insight_types WHERE name=?`, name)
	var id int64
	var n, desc, scope string
	var descVecID sql.NullString
	var categories string
	var targetTokens int
	var analysisMode string
	var lastUpdatedAt, createdAt, updatedAt time.Time
	err := row.Scan(&id, &n, &desc, &descVecID, &categories, &targetTokens,
		&analysisMode, &lastUpdatedAt, &createdAt, &updatedAt, &scope)
	if err != nil {
		return nil, err
	}
	return &memind.MemoryInsightType{
		ID: id, Name: n, Description: desc,
		DescriptionVectorID: descVecID.String,
		Categories:          unmarshalStringSlice(categories),
		TargetTokens:        targetTokens, LastUpdatedAt: lastUpdatedAt,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
		Scope:        memind.MemoryScope(scope),
		AnalysisMode: memind.InsightAnalysisMode(analysisMode),
	}, nil
}

// ListInsightTypes - 列出所有洞察类型
func (o *SQLInsightOps) ListInsightTypes() ([]*memind.MemoryInsightType, error) {
	rows, err := o.db.Query(`SELECT id,name,description,description_vector_id,categories,target_tokens,
		analysis_mode,last_updated_at,created_at,updated_at,scope FROM memory_insight_types`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryInsightType
	for rows.Next() {
		var id int64
		var n, desc, scope string
		var descVecID sql.NullString
		var categories string
		var targetTokens int
		var analysisMode string
		var lastUpdatedAt, createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &n, &desc, &descVecID, &categories, &targetTokens,
			&analysisMode, &lastUpdatedAt, &createdAt, &updatedAt, &scope); err != nil {
			return nil, err
		}
		result = append(result, &memind.MemoryInsightType{
			ID: id, Name: n, Description: desc,
			DescriptionVectorID: descVecID.String,
			Categories:          unmarshalStringSlice(categories),
			TargetTokens:        targetTokens, LastUpdatedAt: lastUpdatedAt,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
			Scope:        memind.MemoryScope(scope),
			AnalysisMode: memind.InsightAnalysisMode(analysisMode),
		})
	}
	return result, nil
}

// UpsertInsights - 插入或更新洞察
func (o *SQLInsightOps) UpsertInsights(memoryID memind.MemoryId, insights []*memind.MemoryInsight) error {
	for _, ins := range insights {
		if ins.CreatedAt.IsZero() {
			ins.CreatedAt = time.Now()
		}
		ins.UpdatedAt = time.Now()
		if ins.ID > 0 {
			_, err := o.db.Exec(`UPDATE memory_insights SET type=?,scope=?,name=?,categories=?,
				group_name=?,last_reasoned_at=?,summary_embedding=?,updated_at=?,tier=?,
				parent_insight_id=?,child_insight_ids=?,version=?,points=?
				WHERE id=?`,
				ins.Type, string(ins.Scope), ins.Name, marshalJSON(ins.Categories),
				nullStr(ins.Group), nullTime(ins.LastReasonedAt),
				marshalJSON(ins.SummaryEmbedding), ins.UpdatedAt, string(ins.Tier),
				nullInt64(ins.ParentInsightID), marshalJSON(ins.ChildInsightIDs),
				ins.Version, marshalJSON(ins.Points), ins.ID)
			if err != nil {
				return err
			}
		} else {
			res, err := o.db.Exec(`INSERT INTO memory_insights(memory_id,type,scope,name,categories,
				group_name,last_reasoned_at,summary_embedding,created_at,updated_at,tier,
				parent_insight_id,child_insight_ids,version,points)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				ins.MemoryID, ins.Type, string(ins.Scope), ins.Name,
				marshalJSON(ins.Categories), nullStr(ins.Group),
				nullTime(ins.LastReasonedAt), marshalJSON(ins.SummaryEmbedding),
				ins.CreatedAt, ins.UpdatedAt, string(ins.Tier),
				nullInt64(ins.ParentInsightID), marshalJSON(ins.ChildInsightIDs),
				ins.Version, marshalJSON(ins.Points))
			if err != nil {
				return err
			}
			if ins.ID == 0 {
				id, _ := res.LastInsertId()
				ins.ID = id
			}
		}
	}
	return nil
}

// GetInsight - 按 ID 获取洞察
func (o *SQLInsightOps) GetInsight(memoryID memind.MemoryId, insightID int64) (*memind.MemoryInsight, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE id=?`, insightID)
	return scanInsight(row)
}

// ListInsights - 列出指定 memory 的所有洞察
func (o *SQLInsightOps) ListInsights(memoryID memind.MemoryId) ([]*memind.MemoryInsight, error) {
	rows, err := o.db.Query(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=?`, memoryID.Identifier())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryInsight
	for rows.Next() {
		ins, err := scanInsight(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ins)
	}
	return result, nil
}

// GetInsightsByType - 按类型列出洞察
func (o *SQLInsightOps) GetInsightsByType(memoryID memind.MemoryId, insightType string) ([]*memind.MemoryInsight, error) {
	rows, err := o.db.Query(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=? AND type=?`, memoryID.Identifier(), insightType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryInsight
	for rows.Next() {
		ins, err := scanInsight(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ins)
	}
	return result, nil
}

// GetInsightsByTier - 按层级列出洞察
func (o *SQLInsightOps) GetInsightsByTier(memoryID memind.MemoryId, tier memind.InsightTier) ([]*memind.MemoryInsight, error) {
	rows, err := o.db.Query(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=? AND tier=?`, memoryID.Identifier(), string(tier))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*memind.MemoryInsight
	for rows.Next() {
		ins, err := scanInsight(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ins)
	}
	return result, nil
}

// GetLeafByGroup - 按类型+分组名获取 Leaf 层级的洞察
func (o *SQLInsightOps) GetLeafByGroup(memoryID memind.MemoryId, insightType, groupName string) (*memind.MemoryInsight, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=? AND type=? AND name=? AND tier='LEAF' LIMIT 1`,
		memoryID.Identifier(), insightType, groupName)
	ins, err := scanInsight(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ins, err
}

// GetBranchByType - 按类型获取 Branch 层级的洞察
func (o *SQLInsightOps) GetBranchByType(memoryID memind.MemoryId, typeName string) (*memind.MemoryInsight, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=? AND type=? AND tier='BRANCH' LIMIT 1`,
		memoryID.Identifier(), typeName)
	ins, err := scanInsight(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ins, err
}

// GetRootByType - 按类型获取 Root 层级的洞察
func (o *SQLInsightOps) GetRootByType(memoryID memind.MemoryId, rootTypeName string) (*memind.MemoryInsight, error) {
	row := o.db.QueryRow(`SELECT id,memory_id,type,scope,name,categories,group_name,last_reasoned_at,
		summary_embedding,created_at,updated_at,tier,parent_insight_id,child_insight_ids,version,points
		FROM memory_insights WHERE memory_id=? AND type=? AND tier='ROOT' LIMIT 1`,
		memoryID.Identifier(), rootTypeName)
	ins, err := scanInsight(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ins, err
}

// DeleteInsights - 批量删除洞察
func (o *SQLInsightOps) DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error {
	if len(insightIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(insightIDs))
	args := make([]any, len(insightIDs))
	for i, id := range insightIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(`DELETE FROM memory_insights WHERE id IN(%s) AND memory_id=?`,
		strings.Join(placeholders, ","))
	args = append(args, memoryID.Identifier())
	_, err := o.db.Exec(q, args...)
	return err
}

// defaultInsightTypes - 返回默认洞察类型列表（在 Init 时自动插入）
func (o *SQLInsightOps) defaultInsightTypes() []*memind.MemoryInsightType {
	now := time.Now()
	return []*memind.MemoryInsightType{
		{Name: "identity", Scope: memind.ScopeUser, Categories: []string{"profile"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "preferences", Scope: memind.ScopeUser, Categories: []string{"profile"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "relationships", Scope: memind.ScopeUser, Categories: []string{"profile"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "experiences", Scope: memind.ScopeUser, Categories: []string{"event"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "behavior", Scope: memind.ScopeUser, Categories: []string{"behavior"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "directives", Scope: memind.ScopeAgent, Categories: []string{"directive"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "playbooks", Scope: memind.ScopeAgent, Categories: []string{"playbook"}, TargetTokens: 500, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "resolutions", Scope: memind.ScopeAgent, Categories: []string{"resolution"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "profile", Scope: memind.ScopeUser, Categories: []string{"profile", "behavior", "event"}, TargetTokens: 800, AnalysisMode: memind.AnalysisModeRoot, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "interaction", Scope: memind.ScopeAgent, Categories: []string{"tool", "directive", "playbook", "resolution"}, TargetTokens: 800, AnalysisMode: memind.AnalysisModeRoot, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
	}
}

// ---------- helpers ----------

// nullStr - 将空字符串转换为 sql.NullString
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullTime - 将可选时间转换为 sql.NullTime
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// nullInt64 - 将可选 int64 转换为 sql.NullInt64
func nullInt64(v any) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	switch x := v.(type) {
	case *int64:
		if x == nil {
			return sql.NullInt64{}
		}
		return sql.NullInt64{Int64: *x, Valid: true}
	case int64:
		return sql.NullInt64{Int64: x, Valid: true}
	}
	return sql.NullInt64{}
}
