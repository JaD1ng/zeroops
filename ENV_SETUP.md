# 配置文件与环境变量指南

## 概述

ZeroOps 现已支持通过配置文件集中管理参数（推荐）。同时也兼容环境变量方式。

## 使用配置文件（推荐）

1) 复制示例并编辑

```bash
cp config.example.json config.json
vi config.json
```

2) 以配置文件启动服务

```bash
go run ./cmd/zeroops -f $(pwd)/config.json
```

配置文件中的字段包括：`server`、`logging`、`database`、`redis`、`alerting.healthcheck/prometheus/remediation/ruleset/receiver`。

## 方法1：使用 .env 文件（可选）

### 步骤1：创建 .env 文件
在项目根目录创建 `.env` 文件：

```bash
# Windows
copy env_example.txt .env

# macOS/Linux
cp env_example.txt .env
```

### 步骤2：编辑 .env 文件
用文本编辑器打开 `.env` 文件，填入你的 API 密钥和配置：

```env
# OpenAI API Key (必需)
OPENAI_API_KEY=sk-your-actual-api-key-here

# 或者使用 Qiniu API (替代方案)
# QINIU_API_KEY=your_qiniu_api_key_here
# QINIU_ENDPOINT=https://openai.qiniu.com/v1

# 评估器模型配置 (必需)
EVALUATOR_LLM=gpt:gpt-oss-20b

# 搜索后端配置 (可选)
SEARCH_BACKEND=DuckDuckGo

# 终端显示配置 (可选)
NO_FORCE_TERMINAL=false
```

### 步骤3：保存文件
保存 `.env` 文件到项目根目录。

## 方法2：设置系统环境变量

### Windows
```cmd
set OPENAI_API_KEY=sk-your-actual-api-key-here
set EVALUATOR_LLM=gpt:gpt-4o-mini
set SEARCH_BACKEND=DuckDuckGo
set NO_FORCE_TERMINAL=false
```

### macOS/Linux
```bash
export OPENAI_API_KEY=sk-your-actual-api-key-here
export EVALUATOR_LLM=gpt:gpt-4o-mini
export SEARCH_BACKEND=DuckDuckGo
export NO_FORCE_TERMINAL=false
```

## Alerting 服务环境变量（数据库 + Webhook 鉴权）

用于接收 Alertmanager Webhook 并将事件入库。

### macOS/Linux
```bash
# 数据库连接（示例：本机 Docker Postgres）
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=zeroops
export DB_SSLMODE=disable

# Webhook 鉴权（与 Alertmanager http_config 对齐，二选一）
# 1) Basic Auth
export ALERT_WEBHOOK_BASIC_USER=alert
export ALERT_WEBHOOK_BASIC_PASS=REDACTED
# 2) Bearer Token（如使用该方式，注释掉上面的 Basic）
# export ALERT_WEBHOOK_BEARER=your_token_here
```

### Windows（PowerShell）
```powershell
$env:DB_HOST="localhost"
$env:DB_PORT="5432"
$env:DB_USER="postgres"
$env:DB_PASSWORD="postgres"
$env:DB_NAME="zeroops"
$env:DB_SSLMODE="disable"

# Basic Auth
$env:ALERT_WEBHOOK_BASIC_USER="alert"
$env:ALERT_WEBHOOK_BASIC_PASS="REDACTED"
# 或 Bearer
# $env:ALERT_WEBHOOK_BEARER="your_token_here"
```

> 启动服务后，可用 README 中的 curl 示例向 `/v1/integrations/alertmanager/webhook` 发送事件并在数据库中验证。

## 环境变量详细说明

### 必需配置

#### `OPENAI_API_KEY`
**描述**: OpenAI API 密钥，用于访问 GPT 模型
**格式**: `sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
**获取方式**: [OpenAI Platform](https://platform.openai.com/api-keys)
**示例**: `OPENAI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

#### `EVALUATOR_LLM`
**描述**: 评估器使用的语言模型
**格式**: 支持多种格式
**选项**:
- `gpt:model_name` - OpenAI GPT 模型
- `manual` - 手动输入模式
- `fake` - 假响应模式（默认，用于测试）
- `http://endpoint` - 自定义 HTTP API 端点

### 可选配置

#### `QINIU_API_KEY` 和 `QINIU_ENDPOINT`
**描述**: 七牛云 API 配置，作为 OpenAI 的替代方案
**格式**: 
- `QINIU_API_KEY=qiniu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`
- `QINIU_ENDPOINT=https://openai.qiniu.com/v1`
**默认端点**: `https://openai.qiniu.com/v1`
**备用端点**: `https://api.qnaigc.com/v1`

#### `SEARCH_BACKEND`
**描述**: 搜索工具使用的后端搜索引擎
**默认值**: `DuckDuckGo`
**选项**: 支持任何兼容的搜索后端
**示例**: `SEARCH_BACKEND=DuckDuckGo`

#### `NO_FORCE_TERMINAL`
**描述**: 控制是否强制使用终端样式
**默认值**: `false`
**选项**: `true` 或 `false`
**用途**: 在非终端环境中禁用富文本样式

## 支持的模型配置

### OpenAI 官方模型
```env
# GPT-4 系列
EVALUATOR_LLM=gpt:gpt-4o-mini
EVALUATOR_LLM=gpt:gpt-4o
EVALUATOR_LLM=gpt:gpt-4-turbo

# GPT-3.5 系列
EVALUATOR_LLM=gpt:gpt-3.5-turbo
EVALUATOR_LLM=gpt:gpt-3.5-turbo-16k
```

### 开源大模型
```env
# GPT-OSS 系列
EVALUATOR_LLM=gpt:gpt-oss-120b
EVALUATOR_LLM=gpt:gpt-oss-20b

# DeepSeek 系列
EVALUATOR_LLM=gpt:deepseek-v3

# GLM 系列
EVALUATOR_LLM=gpt:glm-4.5
EVALUATOR_LLM=gpt:glm-4.5-air

# Qwen 系列
EVALUATOR_LLM=gpt:qwen-turbo
EVALUATOR_LLM=gpt:qwen-max-2025-01-25
EVALUATOR_LLM=gpt:qwen3-32b

# 其他模型
EVALUATOR_LLM=gpt:kimi-k2
EVALUATOR_LLM=gpt:MiniMax-M1
EVALUATOR_LLM=gpt:nvidia/llama-3.3-nemotron-super-49b-v1.5
```

## 验证设置

### 基础验证
运行测试来验证环境变量是否正确加载：

```bashs
python -m agents.base.model
```

如果看到 "✅ API key found: sk-xxxxx..." 说明设置成功。

### 评估器验证
测试评估器是否正常工作：

```bash
python -c "from agents.evaluator import Evaluator; e = Evaluator(); print('✅ Evaluator 初始化成功！')"
```
