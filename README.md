# reminders

统一提醒中心：使用 Go + Vue 3 构建，支持提醒清单、一次性提醒、循环提醒、Cron 规则、多个通知方式和 Bark。通知接收者可以按提醒单独选择，并支持测试发送。项目支持外部 MySQL 持久化，同时提供微信小程序端源码。

## 界面截图

以下截图使用演示数据，仅用于展示界面。

| PC 端计划视图 | PC 端通知方式 |
| --- | --- |
| ![计划视图](images/screenshots/reminders-pc-planned.jpg) | ![通知方式](images/screenshots/reminders-pc-channels.jpg) |

| 移动端计划视图 | 移动端通知方式 |
| --- | --- |
| ![移动端计划视图](images/screenshots/reminders-mobile-planned.jpg) | ![移动端通知方式](images/screenshots/reminders-mobile-channels.jpg) |

## Docker 部署

Compose 只启动提醒应用，MySQL 使用外部服务，不会在 Compose 中额外创建 MySQL 容器。

### 1. 准备 MySQL

先创建数据库和专用用户，并授予该数据库权限。示例：

```sql
CREATE DATABASE IF NOT EXISTS reminders
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'reminders'@'%' IDENTIFIED BY '请替换为实际密码';
GRANT ALL PRIVILEGES ON reminders.* TO 'reminders'@'%';
FLUSH PRIVILEGES;
```

确认 NAS 或 Docker 容器可以访问 MySQL 的 3306 端口。

### 2. 创建 Compose 文件

在 NAS 上创建目录，例如 `/vol1/1000/system/docker/reminders`，并在目录中创建 `docker-compose.yml`：

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
      # 外部 MySQL，根据实际环境修改
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

应用首次启动会自动初始化 `reminders` 数据库中的表，不会迁移或覆盖其他提醒应用的数据。

### 3. 启动

```bash
cd /vol1/1000/system/docker/reminders
docker compose config
docker compose pull
docker compose up -d
docker compose ps
```

打开：

```text
http://NAS_IP:8906
```

第一次打开先注册账号，第一个注册用户自动成为管理员。

### 4. 查看状态和日志

```bash
docker compose ps
docker compose logs --tail=100 reminders
curl http://127.0.0.1:8906/api/version
```

正常状态应为 `running`，健康检查应为 `healthy`，接口会返回应用版本信息。

### 5. 更新镜像

```bash
cd /vol1/1000/system/docker/reminders
docker compose pull
docker compose up -d
```

更新不会删除外部 MySQL 数据，也不会删除 `./data`。不要使用 `docker compose down -v`。

### 6. 持久化

- MySQL 保存用户、提醒、通知方式、接收者、投递任务和操作日志；
- `./data` 保存应用日志和本地数据；
- Compose 只需要一个应用目录挂载：`./data:/app/data`；
- 升级前不要删除数据库或 `data` 目录。

### 7. 镜像架构

`stevensign/reminders:latest` 发布为多架构镜像：

- `linux/amd64`：x86/x86_64 飞牛 NAS；
- `linux/arm64`：ARM64 主机。

查看主机架构：

```bash
docker version --format '{{.Server.Os}}/{{.Server.Arch}}'
```

## 本地开发

```bash
make dev
```

本地开发默认使用 SQLite，访问 `http://localhost:8906`。Docker 部署使用外部 MySQL。

## 微信小程序

小程序源码位于 [`miniapp/`](miniapp/)，说明见 [`miniapp/README.md`](miniapp/README.md)。
