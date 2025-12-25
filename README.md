# ZhuLink (竹林)

**ZhuLink** 是一个去算法推荐的内容聚合社区平台,灵感来源于 Hacker News。

> "在浩瀚的互联网中,信息如同散落的竹叶,繁多而难以捕捉。ZhuLink 意为将这些零散的信息汇聚成一片生机勃勃的竹林。"

## 🌟 核心理念

- **去算法推荐**: 依靠用户共识(投票)挑选有价值的内容,拒绝算法控制
- **时间衰减排序**: 采用类似 Hacker News 的 `(P-1) / (T+2)^G` 排名算法
- **竹林美学**: 清新护眼的绿色主题设计

## ✨ 主要功能

### 📝 社区论坛
- **内容发布**: 支持 URL 链接和 Markdown 文本两种发布方式
- **节点分类**: 按主题节点组织内容,方便浏览和管理
- **无层级评论**: 支持 Markdown 的无层级评论
- **投票系统**: 点赞/踩功能,影响内容排名
- **收藏功能**: 收藏感兴趣的文章,方便后续查看

### 📰 RSS 阅读器
- **订阅管理**: 订阅和管理 RSS/Atom 源
- **分类组织**: 自定义分类管理订阅源
- **定时拉取**: 每 30 分钟自动拉取所有订阅源的新文章
- **定时清除**: 每天凌晨 2 点自动清除发布时间超过 30 天的文章
- **内容推荐**: 一键将优质 RSS 文章推荐到社区
- **订阅限制**: 基于用户积分的订阅数量限制(1000+ 积分无限制)

### 🤖 AI 集成
- **内容审核**: 使用 LLM (Gemini) 自动审核不适宜内容
- **智能摘要**: 为文章生成摘要和关键词
- **SEO 优化**: 自动生成 meta 描述和结构化数据

### 👥 用户系统
- **账号注册**: 邮箱注册,密码 Bcrypt 加密
- **Google OAuth**: 支持 Google 账号登录和绑定
- **积分系统**: 完善的积分奖惩系统
- **通知中心**: 评论回复、点赞、系统通知等
- **个人主页**: 展示用户发布的内容和活动

### 🛡️ 管理功能
- **内容管理**: 置顶、移动、删除帖子
- **用户管理**: 禁言、封禁用户
- **举报系统**: 用户举报 + 管理员审核处理
- **管理员权限**: 基于角色的权限控制

### 🔍 SEO 优化
- **Robots.txt**: 搜索引擎爬虫指引
- **Sitemap.xml**: 动态生成站点地图
- **Meta 标签**: 完整的 SEO meta 标签
- **Open Graph**: 社交媒体分享优化
- **JSON-LD**: 结构化数据支持

### 🎨 用户体验
- **HTMX 交互**: 无刷新页面更新,提升交互体验
- **Alpine.js**: 轻量级前端逻辑处理
- **响应式设计**: 适配各种屏幕尺寸
- **竹林主题**: 精心设计的绿色护眼配色

## 🛠 技术栈

### 后端
- **语言**: Go 1.25+
- **Web 框架**: Gin
- **ORM**: GORM
- **数据库**: PostgreSQL
- **Session**: Cookie-based sessions
- **认证**: Bcrypt + Google OAuth 2.0

### 前端
- **模板引擎**: Go Templates (SSR)
- **交互**: HTMX
- **脚本**: Alpine.js
- **样式**: Tailwind CSS

### 工具库
- **Markdown**: goldmark (解析) + bluemonday (XSS 过滤)
- **RSS**: gofeed
- **爬虫**: go-readability
- **热重载**: Air
- **构建**: Makefile

### 外部服务
- **LLM**: Google Gemini API
- **OAuth**: Google OAuth 2.0

## 🚀 快速开始

### 前置要求
- Go 1.21+
- PostgreSQL
- Node.js & npm (用于构建 Tailwind CSS)

### 1. 克隆项目
```bash
git clone https://github.com/wangtwothree/zhulink.git
cd zhulink
```

### 2. 配置环境变量
复制示例配置文件并修改:
```bash
cp .env.example .env
```

编辑 `.env` 文件,配置以下内容:
```bash
# 数据库配置
DATABASE_URL="host=localhost user=postgres password=yourpassword dbname=zhulink port=5432 sslmode=disable TimeZone=Asia/Shanghai"

# 服务器配置
PORT=8080
GIN_MODE=release  # debug, release, or test
SESSION_SECRET="your-secret-key-change-me"

# 站点配置
SITE_URL="https://zhulink.com"

# LLM 配置 (可选)
LLM_BASE_URL="https://generativelanguage.googleapis.com/v1beta/openai/"
LLM_MODEL="gemini-1.5-flash"
LLM_TOKEN="your-gemini-api-key"

# Google OAuth 配置 (可选)
GOOGLE_CLIENT_ID="your-google-client-id"
GOOGLE_CLIENT_SECRET="your-google-client-secret"
```

### 3. 安装依赖
```bash
# Go 依赖
go mod tidy

# 前端构建工具
npm install -D tailwindcss

# 安装 Air (可选,用于热重载)
go install github.com/air-verse/air@latest
```

### 4. 运行开发环境
使用 Make 命令一键启动:
```bash
make dev
```

这会同时启动:
- Air (后端热重载)
- Tailwind CSS (监听模式)

访问: `http://localhost:8080`

## 🐳 Docker 部署 (推荐生产环境)

### 快速部署

```bash
# 1. 配置环境变量
cp .env.example .env
nano .env  # 修改必要的配置

# 2. 启动服务
docker-compose up -d

# 3. 查看日志
docker-compose logs -f
```

### Docker 特性

- ✅ **多阶段构建**: 优化镜像体积 (< 30MB)
- ✅ **自动化构建**: 自动编译 Go 和压缩 CSS
- ✅ **环境变量映射**: 宿主机 `.env` 文件直接映射到容器
- ✅ **健康检查**: 自动监控服务状态
- ✅ **安全运行**: 非 root 用户运行

### 配置说明

编辑 `.env` 文件,配置数据库连接:

```bash
# 使用已有的 PostgreSQL 数据库
DATABASE_URL="host=your-db-host user=your-user password=your-password dbname=zhulink port=5432 sslmode=disable TimeZone=Asia/Shanghai"

# 如果数据库在本地
DATABASE_URL="host=host.docker.internal user=postgres password=postgres dbname=zhulink port=5432 sslmode=disable TimeZone=Asia/Shanghai"
```

### 开发环境 (仅数据库)

```bash
# 启动开发数据库
docker-compose -f docker-compose.dev.yml up -d

# 本地运行应用
make dev
```

**详细部署文档**: 查看 [DOCKER_DEPLOY.md](./DOCKER_DEPLOY.md)


### 5. 生产环境构建
```bash
# 构建二进制文件
make build

# 运行
./tmp/main
```

## 📂 项目结构

```text
zhulink/
├── cmd/
│   └── server/           # 程序入口
│       └── main.go       # 主程序,路由注册,模板加载
├── internal/
│   ├── db/               # 数据库连接和初始化
│   ├── handlers/         # HTTP 处理器
│   │   ├── auth.go       # 登录/注册
│   │   ├── google_oauth.go  # Google OAuth
│   │   ├── story.go      # 帖子管理
│   │   ├── vote.go       # 投票系统
│   │   ├── bookmark.go   # 收藏功能
│   │   ├── user.go       # 用户管理
│   │   ├── notification.go  # 通知中心
│   │   ├── rss.go        # RSS 阅读器
│   │   ├── transplant.go # RSS 推荐到社区
│   │   ├── admin.go      # 管理员功能
│   │   ├── seo.go        # SEO 相关
│   │   └── ...
│   ├── middleware/       # 中间件
│   │   └── auth.go       # 用户认证中间件
│   ├── models/           # GORM 数据模型
│   │   ├── user.go       # 用户模型
│   │   ├── post.go       # 帖子模型
│   │   ├── comment.go    # 评论模型
│   │   ├── vote.go       # 投票模型
│   │   ├── bookmark.go   # 收藏模型
│   │   ├── notification.go  # 通知模型
│   │   ├── feed.go       # RSS 订阅模型
│   │   ├── report.go     # 举报模型
│   │   └── ...
│   ├── router/           # 路由注册
│   ├── services/         # 业务服务
│   │   ├── ranking.go    # 排名算法服务
│   │   ├── points.go     # 积分系统
│   │   ├── llm.go        # LLM 集成
│   │   ├── rss_fetcher.go  # RSS 抓取
│   │   └── crawler.go    # 网页爬虫
│   └── utils/            # 工具函数
│       ├── hash.go       # 密码加密
│       ├── markdown.go   # Markdown 渲染
│       └── ...
├── web/
│   ├── assets/           # 源文件
│   │   └── input.css     # Tailwind 输入文件
│   ├── static/           # 静态资源
│   │   ├── css/          # 编译后的 CSS
│   │   └── ...
│   └── templates/        # HTML 模板
│       ├── layouts/      # 布局模板
│       ├── includes/     # 公共组件
│       ├── components/   # 可复用组件
│       └── views/        # 页面视图
│           ├── auth/     # 登录/注册
│           ├── story/    # 帖子相关
│           ├── user/     # 用户页面
│           ├── dashboard/  # 个人中心
│           ├── rss/      # RSS 阅读器
│           ├── admin/    # 管理后台
│           └── ...
├── .env.example          # 环境变量示例
├── .air.toml             # Air 配置
├── tailwind.config.js    # Tailwind 配置
├── Makefile              # 构建脚本
└── go.mod                # Go 模块定义
```

## 🔧 Make 命令

```bash
# 开发模式 (热重载 + CSS 监听)
make dev

# 生产构建 (Go 二进制 + 压缩 CSS)
make build

# 仅编译 CSS (开发模式,监听)
make css-watch

# 仅编译 CSS (生产模式,压缩)
make css-build

# 安装开发工具
make setup

# 清理构建产物
make clean
```

## 🎯 核心特性说明

### 排名算法
采用时间衰减算法,确保新内容有机会展示,同时优质内容能够保持较高排名:
```
Score = (Points - 1) / (HoursSincePost + 2)^Gravity
```
- Points: 净赞数 (赞 - 踩)
- HoursSincePost: 发布后经过的小时数
- Gravity: 重力系数 (默认 1.8)

### 安全机制
- **密码加密**: 使用 Bcrypt 加密存储
- **XSS 防护**: Markdown 内容使用 bluemonday 过滤
- **CSRF 防护**: Session-based 认证
- **SQL 注入防护**: GORM 参数化查询
- **内容审核**: LLM 自动审核不适宜内容

### 数据管理
- **硬删除**: 删除数据直接从数据库移除
- **级联删除**: 删除帖子时自动删除相关评论、投票等
- **事务处理**: 关键操作使用数据库事务保证一致性
- **RSS 定时任务**:
  - 每 30 分钟自动拉取所有订阅源的新文章
  - 每天凌晨 2 点自动清除发布时间超过 30 天的文章
  - 服务启动时立即执行一次拉取

## 📊 积分系统

| 操作 | 积分变化 |
|------|---------|
| 发布帖子 | +5 |
| 发表评论 | +2 |
| 收到点赞 | +1 |
| 收到踩 | -1 |

积分用途:
- 影响 RSS 订阅数量限制
- 未来可扩展更多权限和功能

## 🔐 权限系统

### 用户角色
- **普通用户**: 发帖、评论、投票、收藏
- **管理员**: 所有普通用户权限 + 管理功能

### 管理员权限
- 置顶/取消置顶帖子
- 移动帖子到其他节点
- 删除任意帖子/评论
- 禁言/封禁用户
- 处理用户举报

## 🌐 部署建议

### 推荐方式: Docker Compose (生产环境)

使用 Docker Compose 部署是最简单和推荐的方式:

```bash
# 配置环境变量
cp .env.production.example .env
nano .env

# 启动服务
docker-compose up -d
```

**优势**:
- 一键部署,无需手动配置环境
- 自动构建优化的镜像 (< 30MB)
- 使用已有的 PostgreSQL 数据库
- 包含健康检查和自动重启
- `.env` 文件直接映射到容器

详见: [DOCKER_DEPLOY.md](./DOCKER_DEPLOY.md)

### 传统部署方式

#### 环境要求
- PostgreSQL 12+
- 至少 512MB RAM
- Go 1.21+ (编译环境)

#### 生产环境配置
1. 设置 `GIN_MODE=release`
2. 使用强随机 `SESSION_SECRET`
3. 配置 HTTPS (推荐使用 Nginx 反向代理)
4. 配置数据库连接池
5. 设置合适的超时时间

### 性能优化
- 使用 Redis 缓存热点数据 (可选)
- 配置 CDN 加速静态资源
- 数据库索引优化
- 启用 Gzip 压缩

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request!

### 开发流程
1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码规范
- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 添加必要的注释
- 编写单元测试

## 📝 License

MIT License

## 🙏 致谢

- 灵感来源: [Hacker News](https://news.ycombinator.com/)
- 排名算法参考: [How Hacker News ranking algorithm works](https://medium.com/hacking-and-gonzo/how-hacker-news-ranking-algorithm-works-1d9b0cf2c08d)

## 📧 联系方式

如有问题或建议,欢迎通过以下方式联系:
- GitHub Issues
- Email: [your-email@example.com]

---

**ZhuLink** - 让优质内容自然浮现 🎋
