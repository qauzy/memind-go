# Linux + MySQL 源码部署指南

> 适用于生产环境部署 memind-go 服务。

---

## 目录

1. [环境要求](#1-环境要求)
2. [MySQL 准备](#2-mysql-准备)
3. [编译部署（常规方式）](#3-编译部署常规方式)
4. [编译部署（Docker 方式）](#4-编译部署docker-方式)
5. [运行验证](#5-运行验证)
6. [生产配置建议](#6-生产配置建议)
7. [常见问题](#7-常见问题)

---

## 1. 环境要求

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| Go | 1.25+ | 因依赖 `modernc.org/sqlite` 要求 Go 1.25 |
| MySQL | 8.0+ | 建议 8.0.30+，使用 utf8mb4 字符集 |
| OS | Linux | 推荐 Ubuntu 22.04 / Debian 12 / CentOS 9 |
| 内存 | 512 MB | 空载约 50 MB，随数据量增长 |
| 磁盘 | 1 GB | 主要用于持久化数据和日志 |

确认版本：

```bash
go version
# go version go1.25.0 linux/amd64

mysql --version
# mysql  Ver 8.0.35
```

---

## 2. MySQL 准备

### 2.1 安装 MySQL

Ubuntu / Debian：

```bash
sudo apt update
sudo apt install mysql-server -y
sudo systemctl start mysql
sudo systemctl enable mysql
```

CentOS / RHEL 9：

```bash
sudo dnf install mysql-server -y
sudo systemctl start mysqld
sudo systemctl enable mysqld
```

### 2.2 创建数据库和用户

```bash
sudo mysql
```

```sql
CREATE DATABASE memind
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER 'memind'@'localhost' IDENTIFIED BY '你的强密码';

GRANT ALL PRIVILEGES ON memind.* TO 'memind'@'localhost';

FLUSH PRIVILEGES;
EXIT;
```

> **安全提醒**：生产环境不要用 root 用户连接，密码强度至少 16 位含大小写+数字+特殊字符。

### 2.3 验证连接

```bash
mysql -u memind -p -D memind -e "SELECT 1 AS test;"
```

---

## 3. 编译部署（常规方式）

### 3.1 克隆代码

```bash
git clone https://github.com/openmemind/memind-go.git
cd memind-go
```

### 3.2 下载依赖

```bash
go mod download
```

`go-sql-driver/mysql` 会自动下载。

### 3.3 编译自定义入口（使用 MySQL 存储）

默认的 `cmd/memind/main.go` 使用内存存储。部署到生产需改用 MySQL，有两种方式：

#### 方式 A：创建生产入口文件（推荐）

创建 `cmd/memind-mysql/main.go`：

```go
package main

import (
	"flag"
	"log"
	"os"

	"github.com/openmemind/memind-go/engine"
	"github.com/openmemind/memind-go/server"
	sqlstore "github.com/openmemind/memind-go/store/sql"
)

func main() {
	addr := flag.String("addr", ":8018", "server listen address")
	dsn := flag.String("dsn", "", "MySQL DSN (e.g. user:pass@tcp(host:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("memind-go starting with MySQL...")

	// 环境变量覆盖
	if addrEnv := os.Getenv("MEMIND_ADDR"); addrEnv != "" {
		*addr = addrEnv
	}
	if dsnEnv := os.Getenv("MYSQL_DSN"); dsnEnv != "" {
		*dsn = dsnEnv
	}

	store, err := sqlstore.NewMySQLStore(*dsn)
	if err != nil {
		log.Fatalf("failed to init MySQL store: %v", err)
	}

	memory := engine.Builder().
		Store(store).
		Build()

	srv := server.New(memory, *addr)
	log.Printf("listening on %s", *addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

#### 方式 B：修改原有 main.go

修改 `cmd/memind/main.go`，在 import 中添加 `sqlstore`，将 `store.NewInMemoryStore()` 替换为 `sqlstore.NewMySQLStore(dsn)`。

#### 3.4 编译

```bash
# 使用方式 A
go build -o memind-server ./cmd/memind-mysql/

# 或使用方式 B
# go build -o memind-server ./cmd/memind/
```

编译产物为单一二进制文件 `memind-server`，无外部依赖。

#### 3.5 运行

```bash
# 通过环境变量配置
export MYSQL_DSN="memind:你的强密码@tcp(127.0.0.1:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
export MEMIND_ADDR=":8018"

./memind-server
```

或通过命令行参数：

```bash
./memind-server \
  -addr :8018 \
  -dsn "memind:你的强密码@tcp(127.0.0.1:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
```

> **第一次启动**：会自动执行建表迁移（DDL），无需手动导入 SQL。

---

## 4. 编译部署（Docker 方式）

### 4.1 多阶段构建 Dockerfile

创建 `Dockerfile`：

```dockerfile
# ---- 构建阶段 ----
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /memind-server ./cmd/memind-mysql/

# ---- 运行阶段 ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /memind-server /usr/local/bin/memind-server
EXPOSE 8018
ENTRYPOINT ["memind-server"]
```

### 4.2 构建镜像

```bash
docker build -t memind-server:latest .
```

### 4.3 运行容器

```bash
docker run -d \
  --name memind \
  --restart always \
  -p 8018:8018 \
  -e MYSQL_DSN="memind:密码@tcp(主机IP:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local" \
  -e MEMIND_ADDR=":8018" \
  memind-server:latest
```

> MySQL 不在容器内时，需确保 `主机IP` 可从容器访问（使用 host 网络或 docker-compose）。

### 4.4 docker-compose 方案

创建 `docker-compose.yml`：

```yaml
version: "3.9"

services:
  mysql:
    image: mysql:8.0
    container_name: memind-mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: root密码
      MYSQL_DATABASE: memind
      MYSQL_USER: memind
      MYSQL_PASSWORD: 你的强密码
    volumes:
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5

  memind:
    build: .
    container_name: memind-server
    restart: always
    ports:
      - "8018:8018"
    environment:
      MYSQL_DSN: "memind:你的强密码@tcp(mysql:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
      MEMIND_ADDR: ":8018"
    depends_on:
      mysql:
        condition: service_healthy

volumes:
  mysql-data:
```

启动：

```bash
docker compose up -d
```

---

## 5. 运行验证

### 5.1 健康检查（服务正在运行）

```bash
curl -s http://127.0.0.1:8018/open/v1/memory/sync/extract \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"memoryId":{"userId":"test"},"content":{"type":"text","content":"hello world"}}'
```

返回 `200 OK` 即服务正常。

### 5.2 完整流程测试

```bash
# 1. 添加消息（注意 content 为数组，每项含 type+text）
curl -s http://127.0.0.1:8018/open/v1/memory/sync/add-message \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"memoryId":{"userId":"user1"},"message":{"role":"user","content":[{"type":"text","text":"我喜欢吃披萨"}]}}'

# 2. 提交（触发提取 + 洞察树）
curl -s http://127.0.0.1:8018/open/v1/memory/sync/commit \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"memoryId":{"userId":"user1"}}'

# 3. 检索
curl -s http://127.0.0.1:8018/open/v1/memory/retrieve \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"memoryId":{"userId":"user1"},"query":"披萨"}'
```

### 5.3 验证数据已持久化

```bash
mysql -u memind -p -D memind -e "SELECT COUNT(*) AS items FROM memory_raw_data;"
mysql -u memind -p -D memind -e "SELECT COUNT(*) AS insights FROM memory_insights;"
```

重启服务后再次查询，数据应仍然存在。

---

## 6. 生产配置建议

### 6.1 Systemd 服务

创建 `/etc/systemd/system/memind.service`：

```ini
[Unit]
Description=memind-go Memory Service
After=network.target mysql.service
Requires=mysql.service

[Service]
Type=simple
User=memind
Group=memind
ExecStart=/usr/local/bin/memind-server -addr :8018
Environment=MYSQL_DSN="memind:密码@tcp(127.0.0.1:3306)/memind?charset=utf8mb4&parseTime=true&loc=Local"
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

创建系统用户并部署：

```bash
sudo useradd -r -s /bin/false memind
sudo cp memind-server /usr/local/bin/
sudo chmod +x /usr/local/bin/memind-server
sudo chown memind:memind /usr/local/bin/memind-server
sudo systemctl daemon-reload
sudo systemctl enable memind --now
```

查看日志：

```bash
sudo journalctl -u memind -f
```

### 6.2 MySQL 连接池优化

`NewMySQLStore` 默认连接池配置（`store/sql/mysql.go:16-21`）：

```go
db.SetMaxOpenConns(10)    // 最大连接数
db.SetMaxIdleConns(5)     // 最大空闲连接数
```

根据服务器并发量调整。MySQL 侧建议：

```sql
-- 检查 MySQL 最大连接数
SHOW VARIABLES LIKE 'max_connections';
-- 建议设为 200+
SET GLOBAL max_connections = 200;
```

### 6.3 性能监控

```sql
-- 查看表大小
SELECT
  table_name,
  ROUND(((data_length + index_length) / 1024 / 1024), 2) AS size_mb
FROM information_schema.tables
WHERE table_schema = 'memind'
ORDER BY size_mb DESC;

-- 查看慢查询
SHOW FULL PROCESSLIST;
```

---

## 7. 常见问题

### Q1：启动报错 `dial tcp 127.0.0.1:3306: connect: connection refused`

MySQL 未启动或监听端口不对。

```bash
# 检查 MySQL 状态
sudo systemctl status mysql

# 检查监听端口
sudo ss -tlnp | grep 3306

# 确保 MySQL 在 127.0.0.1 监听（而非仅 ::1）
sudo grep bind-address /etc/mysql/mysql.conf.d/mysqld.cnf
```

### Q2：启动报错 `Access denied for user 'memind'@'localhost'`

用户名或密码错误。

```bash
# 检查是否能直连
mysql -u memind -p -D memind

# 检查用户授权
sudo mysql -e "SELECT user, host, plugin FROM mysql.user WHERE user='memind';"
sudo mysql -e "SHOW GRANTS FOR 'memind'@'localhost';"

# 如需要重新授权
sudo mysql -e "ALTER USER 'memind'@'localhost' IDENTIFIED BY '新密码';"
```

### Q3：建表时出现 `Specified key was too long`

MySQL 的 InnoDB 前缀索引限制（767 bytes for dynamic row format）。`memory_items.content` 为 TEXT 类型，但索引只建在 `memory_id` 和 `content_hash`，不会触发此问题。如果自定义表结构遇到，检查索引列长度。

### Q4：想切回内存模式调试

直接用 `cmd/memind/main.go` 即可：

```bash
go run ./cmd/memind/ -addr :8018
```

无需 MySQL。

### Q5：重启后数据丢失？

MySQL 模式下数据持久化到数据库，重启不会丢。如果数据丢失，检查：

```bash
# 确认 MySQL 数据文件存在
sudo ls -la /var/lib/mysql/memind/
```

### Q6：如何升级？

```bash
# 拉取最新代码
git pull

# 重新编译
go build -o memind-server ./cmd/memind-mysql/

# 替换二进制
sudo systemctl stop memind
sudo cp memind-server /usr/local/bin/
sudo systemctl start memind

# 迁移是自动的——启动时由 Init() 自动执行，无需手动操作
```

> 数据库迁移由 `store/sql/ddl.go` 中的 `Init()` 方法自动执行（`CREATE TABLE IF NOT EXISTS`），追加新列或新表无需手动干预。如需变更已有列，需手动执行 `ALTER TABLE`。

---

## 附表：DSN 参数参考

MySQL DSN 格式为：

```
user:password@tcp(host:port)/dbname?param1=value1&param2=value2
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `charset` | 否 | `utf8mb4` | 字符集 |
| `parseTime` | 是 | `true` | 自动解析 DATETIME 为 time.Time |
| `loc` | 否 | `Local` | 时区，建议 `Asia/Shanghai` |
| `timeout` | 否 | `30s` | 连接超时 |
| `readTimeout` | 否 | `30s` | 读取超时 |
| `writeTimeout` | 否 | `30s` | 写入超时 |

完整示例：

```
memind:MyStr0ng!Pass@tcp(192.168.1.100:3306)/memind?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai&timeout=10s
```
