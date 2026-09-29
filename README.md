# reminders

基于 Go 和 Vue 3 的统一提醒中心，将提醒清单、循环规则和多种通知方式放在同一个界面中，并支持 Bark、微信小程序和外部 MySQL 持久化。

## 功能

- 今天、计划、全部、已完成和自定义清单；
- 一次性提醒使用公历日期加时间；
- 循环提醒支持每天、每周、每月、每年和自定义 Cron；
- 支持开始时间、结束时间、时间点、时间段和时分秒间隔；
- 间隔严格限制在开始时间和结束时间之间；
- 站内消息、电子邮件、短信、飞书、QQ、钉钉、企业微信、PushPlus、Server 酱、Gotify、Ntfy、IYUU、巴法云和 Bark；
- 每种外部通知方式可以保存多个接收者，创建提醒时逐行选择；
- 每个接收者都可以单独测试发送；
- 测试发送使用当前提醒的标题、备注和时间，当前提醒必填项未完成时不会发送；
- 配置页的发送测试必须填写本次接收者，不会自动保存接收者；
- 需要管理员配置的通知方式显示“已配置/未配置”，未配置的方式在创建提醒时置灰；
- 投递任务持久化、幂等处理、失败分类和重试；
- 响应式 Web 界面和微信小程序端；
- 接收目标在数据库中加密保存。

## 界面截图

以下截图使用演示数据，仅用于展示界面布局和使用方式。

| 计划视图 | 通知方式 |
| --- | --- |
| ![计划视图](images/screenshots/reminders-pc-planned.jpg) | ![通知方式](images/screenshots/reminders-pc-channels.jpg) |

| 移动端计划视图 | 移动端通知方式 |
| --- | --- |
| ![移动端计划视图](images/screenshots/reminders-mobile-planned.jpg) | ![移动端通知方式](images/screenshots/reminders-mobile-channels.jpg) |

## Docker 部署

Docker 部署使用外部 MySQL，Compose 只启动提醒应用，不会额外创建 MySQL 容器。这样可以使用 NAS 上已有的 MySQL，并把数据库数据与应用容器解耦。

### 目录准备

在 NAS 或服务器上创建目录并进入：

```bash
mkdir -p /path/to/reminders/data
cd /path/to/reminders
```

将项目中的 `docker-compose.yml` 放到该目录。Compose 文件当前结构如下，数据库连接字段请按实际环境修改：

```yaml
name: reminders

services:
  reminders:
    image: stevensign/reminders:latest
    container_name: reminders
    restart: unless-stopped
    ports:
      - "8906:8906"
    environment:
      # 外部 MySQL
      - MYSQL_HOST=你的MySQL地址
      - MYSQL_PORT=3306
      - MYSQL_USER=你的数据库用户
      - MYSQL_PASSWORD=你的数据库密码
      - MYSQL_DB_NAME=reminders
      - TZ=Asia/Shanghai
      - DATA_DIR=/app/data
      - DISABLE_STATS=1
    volumes:
      - ./data:/app/data
```

### MySQL 准备

先在外部 MySQL 中创建数据库和专用用户，并授予该数据库的完整权限。示例 SQL 如下，密码请替换为实际值：

```sql
CREATE DATABASE IF NOT EXISTS reminders
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'reminders'@'%' IDENTIFIED BY '请替换为实际密码';
GRANT ALL PRIVILEGES ON reminders.* TO 'reminders'@'%';
FLUSH PRIVILEGES;
```

如果 MySQL 只允许本机访问，还需要在 MySQL 配置中允许 Docker/NAS 网段连接，并确认 3306 端口可访问。应用只使用 `reminders` 数据库，不会迁移或覆盖其他提醒应用的数据。

### 启动

```bash
docker compose config
docker compose pull
docker compose up -d
docker compose ps
```

访问：

```text
http://NAS或服务器IP:8906
```

首次打开时注册账号。第一个注册用户会自动成为管理员。

### 检查日志和健康状态

```bash
docker compose ps
docker compose logs --tail=100 reminders
curl http://127.0.0.1:8906/api/version
```

正常时 `/api/version` 会返回应用版本信息，容器状态应为 `running`，健康检查应为 `healthy`。

### 更新镜像

```bash
docker compose pull
docker compose up -d
docker image prune -f
```

更新只会替换应用容器，不会删除外部 MySQL 数据，也不会删除 `./data`。不要使用 `docker compose down -v`，避免误删 Compose 管理的卷。

### 持久化说明

提醒、用户、通知方式、投递任务和操作日志保存在外部 MySQL；`./data` 保存应用日志及其他本地数据。Compose 中只挂载一个应用数据目录：

```yaml
volumes:
  - ./data:/app/data
```

### 镜像架构

Docker Hub 的 `latest` 发布为多架构镜像：

- `linux/amd64`：适用于 x86/x86_64 飞牛 NAS；
- `linux/arm64`：适用于 ARM64 主机。

查看 Docker 主机架构：

```bash
docker version --format '{{.Server.Os}}/{{.Server.Arch}}'
```

## 通知方式和接收者

进入“通知方式”页面：

- 邮件、短信已配置后可以修改发送服务；
- 修改密码、授权码或 Token 时需要重新填写，敏感字段不会回显；
- 飞书和 QQ 支持更换机器人凭证；
- 站内消息和其他内置渠道不需要管理员配置；
- 未配置的方式不能在创建提醒时选择。

创建或编辑提醒时：

1. 选择通知方式；
2. 点击“＋ 添加接收人”；
3. 每一行选择一个已保存的接收者，或输入新的接收目标；
4. `−` 只将接收者移出当前提醒；
5. “删除”删除当前用户保存的接收者；
6. “测试发送”向这一行的接收者发送当前提醒内容。

外部通知方式至少选择一个接收者后才能保存提醒。

常用接收目标格式：

| 通知方式 | 接收目标 |
| --- | --- |
| 电子邮件 | `name@example.com` |
| 手机短信 | 中国大陆手机号 |
| 飞书 | 工作邮箱、手机号或 OpenID |
| QQ | 用户 OpenID |
| 企业微信应用 | `corpid|corpsecret|agentid|userid` |
| Gotify | `服务地址|应用 Token` |
| Ntfy | `服务地址|Topic|可选 Token` |
| 巴法云 | `UID|Topic` |
| Bark | `服务地址|Device Key|可选分组` |
| 其他 Token 渠道 | Token 或 SendKey |

Webhook 只接受对应平台的官方 HTTPS 地址，避免把应用变成任意 URL 请求代理。

## 微信小程序

小程序源码位于 [`miniapp/`](miniapp/)，支持提醒新增、编辑、循环规则、通知方式、多接收者选择、移出、删除和单独测试发送。

配置说明见 [`miniapp/README.md`](miniapp/README.md)。

## 技术栈

- Go、Gin、GORM；
- MySQL 8（Docker 部署）/ SQLite（单元测试回退）；
- Vue 3、TypeScript、Pinia、Vue Router；
- Tailwind CSS、Vite；
- 微信小程序原生页面。

## 本地开发

要求 Go 1.26+、Node.js 20+、CGO 和本地 C 编译器。

```bash
make dev
```

浏览器访问 `http://localhost:8906`。本地开发默认使用 SQLite，Docker 部署使用外部 MySQL。

构建和启动已构建产物：

```bash
make build
make start
```

## 构建与测试

```bash
cd server && go test ./...
cd web && npm run build
node --check miniapp/pages/edit/edit.js
```

构建本地镜像：

```bash
docker build --pull=false -t reminders:latest .
```

构建并发布多架构镜像：

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t stevensign/reminders:latest --push .
```

版本记录见 [`CHANGELOG.md`](CHANGELOG.md)，技术和接口说明见 [`docs/README.md`](docs/README.md) 与 [`API.md`](API.md)。
