package memind_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---------- WeFlow API 类型 ----------

// WeFlowContact - WeFlow 联系人
type WeFlowContact struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Remark      string `json:"remark"`
}

// weflowContactsResponse - WeFlow /api/v1/contacts 响应
type weflowContactsResponse struct {
	Success  bool            `json:"success"`
	Contacts []WeFlowContact `json:"contacts"`
}

// WeFlowMessage - WeFlow 单条消息
type WeFlowMessage struct {
	CreateTime    int64  `json:"createTime"`
	IsSend        int    `json:"isSend"`
	Content       string `json:"content"`
	ParsedContent string `json:"parsedContent"`
}

// weflowMessagesResponse - WeFlow /api/v1/messages 响应
type weflowMessagesResponse struct {
	Success  bool            `json:"success"`
	Count    int             `json:"count"`
	HasMore  bool            `json:"hasMore"`
	Messages []WeFlowMessage `json:"messages"`
}

// ---------- HTTP 工具 ----------

var httpClient = &http.Client{Timeout: 300 * time.Second}

func weflowGet[T any](url, token string) (*T, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var v T
	return &v, json.NewDecoder(resp.Body).Decode(&v)
}

func weflowPost[T any](url, token string, body any) (*T, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var v T
	return &v, json.NewDecoder(resp.Body).Decode(&v)
}

func memindPost(url string, body any) ([]byte, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, e.Error)
	}
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

// ---------- 主测试 ----------

// TestImportWeChatContact - 通过 WeFlow API 导入某微信好友的聊天记录（从最早开始，每 20 条提交一次）
//
// 环境变量：WEFLOW_TOKEN（必填）, WEFLOW_CONTACT（好友昵称/备注）, WEFLOW_BASE_URL（默认 :5031）, MEMIND_BASE_URL（默认 :8018）
func TestImportWeChatContact(t *testing.T) {
	token := "3f65f5f75811c87d498b372436393c73"
	contactName := "文崔"
	if token == "" || contactName == "" {
		t.Skip("set WEFLOW_TOKEN and WEFLOW_CONTACT")
	}

	weflowBase := strDefault(os.Getenv("WEFLOW_BASE_URL"), "http://127.0.0.1:5031")
	memindBase := strDefault(os.Getenv("MEMIND_BASE_URL"), "http://192.168.199.97:8018")

	// 1. 查找联系人
	t.Logf("查找联系人: %s", contactName)
	cr, err := weflowPost[weflowContactsResponse](
		weflowBase+"/api/v1/contacts", token,
		map[string]string{"keyword": contactName, "limit": "10"},
	)
	if err != nil {
		t.Fatalf("contacts 查询失败: %v", err)
	}
	if len(cr.Contacts) == 0 {
		t.Fatalf("未找到联系人 %q", contactName)
	}
	c := cr.Contacts[0]
	t.Logf("已找到: %s (wxid: %s)", c.DisplayName, c.Username)

	// 2. 读取断点时间戳，只拉取未导入过的消息
	since := loadCheckpoint(c.Username)
	if since > 0 {
		t.Logf("断点时间: %s 之后的消息", time.Unix(since, 0).Format("2006-01-02 15:04:05"))
	} else {
		t.Logf("无断点，拉取全部消息")
	}

	// 拉取消息（GET）
	const fetchLimit = 500
	t.Logf("拉取最近 %d 条消息 GET (since=%d)...", fetchLimit, since)

	var allMessages []WeFlowMessage

	fetchMessages := func(offset int, since int64) (*weflowMessagesResponse, error) {
		url := fmt.Sprintf("%s/api/v1/messages?talker=%s&limit=%d&offset=%d",
			weflowBase, c.Username, fetchLimit, offset)
		if since > 0 {
			url += fmt.Sprintf("&start=%d", since)
		}
		return weflowGet[weflowMessagesResponse](url, token)
	}

	mr, err := fetchMessages(0, since)
	if err != nil {
		t.Fatalf("拉取消息失败: %v", err)
	}
	t.Logf("响应: success=%v count=%d hasMore=%v messages=%d",
		mr.Success, mr.Count, mr.HasMore, len(mr.Messages))

	if len(mr.Messages) == 0 {
		raw, _ := json.Marshal(mr)
		t.Logf("原始响应: %s", string(raw))
		t.Fatalf("该联系人无新消息")
	}
	allMessages = append(allMessages, mr.Messages...)

	// 翻页
	for mr.HasMore {
		mr, err = fetchMessages(len(allMessages), since)
		if err != nil {
			break
		}
		allMessages = append(allMessages, mr.Messages...)
		t.Logf("  已拉取 %d 条...", len(allMessages))
	}
	t.Logf("共拉取 %d 条消息", len(allMessages))

	// 按时间升序（最早的在前）
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].CreateTime < allMessages[j].CreateTime
	})
	t.Logf("时间范围: %s ~ %s",
		time.Unix(allMessages[0].CreateTime, 0).Format("2006-01-02"),
		time.Unix(allMessages[len(allMessages)-1].CreateTime, 0).Format("2006-01-02"),
	)

	// 3. 按时间顺序逐批提交 extract（直接提取，不走缓冲区）
	memID := map[string]string{"userId": "wechat-" + c.Username, "agentId": ""}
	config := map[string]any{"enableInsight": true, "scope": "user", "language": "Chinese"}

	extractURL := memindBase + "/open/v1/memory/sync/extract"
	batchSize := 100
	totalExtracted := 0

	for i := 0; i < len(allMessages); i += batchSize {
		end := i + batchSize
		if end > len(allMessages) {
			end = len(allMessages)
		}
		batch := allMessages[i:end]

		// 拼接本批消息为对话文本
		var conversation strings.Builder
		for _, wfMsg := range batch {
			text := pickText(wfMsg)
			if text == "" {
				continue
			}
			role := c.DisplayName
			if wfMsg.IsSend == 1 {
				role = "我"
			}
			conversation.WriteString(fmt.Sprintf("[%s] %s\n", role, text))
		}
		if conversation.Len() == 0 {
			continue
		}

		// 直接提取
		t.Logf("提交第 %d~%d 条...", i+1, end)
		raw, err := memindPost(extractURL, map[string]any{
			"memoryId": memID,
			"content":  map[string]string{"type": "ConversationContent", "content": conversation.String()},
			"config":   config,
		})
		if err != nil {
			t.Fatalf("extract 失败: %v", err)
		}

		var r struct {
			Status string `json:"status"`
			Items  struct {
				NewItems []any `json:"newItems"`
			} `json:"items"`
			Insights struct {
				Insights []any `json:"insights"`
			} `json:"insights"`
		}
		json.Unmarshal(raw, &r)
		totalExtracted += len(r.Items.NewItems)
		t.Logf("  → status=%s items=%d insights=%d",
			r.Status, len(r.Items.NewItems), len(r.Insights.Insights))

		// 每批成功后保存断点，避免重跑重复
		saveCheckpoint(c.Username, allMessages[end-1].CreateTime)
	}

	t.Logf("导入完成: 共提取 %d 个 item", totalExtracted)

	// 4. 检索验证
	retrieveURL := memindBase + "/open/v1/memory/retrieve"
	raw, err := memindPost(retrieveURL, map[string]any{
		"memoryId": memID,
		"query":    fmt.Sprintf("与 %s 的聊天内容", c.DisplayName),
		"config": map[string]any{
			"tier1": map[string]any{"enabled": true, "topK": 5, "minScore": 0.3},
			"tier2": map[string]any{"enabled": true, "topK": 15, "minScore": 0.0},
			"tier3": map[string]any{"enabled": true, "topK": 5, "minScore": 0.0},
		},
	})
	if err != nil {
		t.Fatalf("检索失败: %v", err)
	}
	var ret struct {
		Items    []any `json:"items"`
		Insights []any `json:"insights"`
		RawData  []any `json:"rawData"`
	}
	json.Unmarshal(raw, &ret)
	t.Logf("检索结果: items=%d insights=%d rawData=%d",
		len(ret.Items), len(ret.Insights), len(ret.RawData))
	if len(ret.Items) == 0 && len(ret.Insights) == 0 {
		t.Fatal("检索结果为空")
	}

}

// ---------- 辅助 ----------

func pickText(m WeFlowMessage) string {
	text := m.ParsedContent
	if text == "" {
		text = m.Content
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "<msg>") {
		return ""
	}
	return text
}

func strDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// checkpointPath - 断点文件路径
func checkpointPath(talker string) string {
	dir, _ := os.UserCacheDir()
	if dir == "" {
		dir = os.TempDir()
	}
	return fmt.Sprintf("%s/memind-weflow-%s.checkpoint", dir, talker)
}

// loadCheckpoint - 读取上次导入的最后消息时间戳
func loadCheckpoint(talker string) int64 {
	data, err := os.ReadFile(checkpointPath(talker))
	if err != nil {
		return 0
	}
	var ts int64
	fmt.Sscanf(string(data), "%d", &ts)
	return ts
}

// saveCheckpoint - 保存最后消息时间戳
func saveCheckpoint(talker string, ts int64) {
	os.WriteFile(checkpointPath(talker), []byte(fmt.Sprintf("%d", ts)), 0644)
}
