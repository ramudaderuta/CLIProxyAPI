# CLI 代理 API

[English](README.md) | 中文

一个为 CLI 提供 OpenAI/Gemini/Claude/Codex/Kiro 兼容 API 接口的代理服务器。

现已支持通过 OAuth 登录接入 OpenAI Codex（GPT 系列）、Claude Code 和 Kiro AI（基于令牌认证）。

您可以使用本地或多账户的CLI方式，通过任何与 OpenAI（包括Responses）/Gemini/Claude/Kiro 兼容的客户端和SDK进行访问。

现已新增国内提供商：[Qwen Code](https://github.com/QwenLM/qwen-code)、[iFlow](https://iflow.cn/)。

## 功能特性

- 为 CLI 模型提供 OpenAI/Gemini/Claude/Codex/Kiro 兼容的 API 端点
- 新增 OpenAI Codex（GPT 系列）支持（OAuth 登录）
- 新增 Claude Code 支持（OAuth 登录）
- 新增 Qwen Code 支持（OAuth 登录）
- 新增 iFlow 支持（OAuth 登录）
- 新增 Kiro AI 支持（基于令牌认证，无需在线 OAuth）
- 支持流式与非流式响应
- 函数调用/工具支持
- 多模态输入（文本、图片）
- 多账户支持与轮询负载均衡（Gemini、OpenAI、Claude、Qwen、iFlow 与 Kiro）
- 简单的 CLI 身份验证流程（Gemini、OpenAI、Claude、Qwen 与 iFlow）
- Kiro AI 基于令牌的身份验证（无需在线 OAuth）
- 支持 Gemini AIStudio API 密钥
- 支持 AI Studio Build 多账户轮询
- 支持 Gemini CLI 多账户轮询
- 支持 Claude Code 多账户轮询
- 支持 Qwen Code 多账户轮询
- 支持 iFlow 多账户轮询
- 支持 OpenAI Codex 多账户轮询
- 通过配置接入上游 OpenAI 兼容提供商（例如 OpenRouter）
- 可复用的 Go SDK（见 `docs/sdk-usage_CN.md`）

## 新手入门
## 安装

### 前置要求

- Go 1.24 或更高版本
- 有权访问 Gemini CLI 模型的 Google 账户（可选）
- 有权访问 OpenAI Codex/GPT 的 OpenAI 账户（可选）
- 有权访问 Claude Code 的 Anthropic 账户（可选）
- 有权访问 Qwen Code 的 Qwen Chat 账户（可选）
- 有权访问 iFlow 的 iFlow 账户（可选）

### 从源码构建

1. 克隆仓库：
   ```bash
   git clone https://github.com/luispater/CLIProxyAPI.git
   cd CLIProxyAPI
   ```

2. 构建应用程序：
   ```bash
   go build -o cli-proxy-api ./cmd/server
   ```

### 通过 Homebrew 安装

```bash
brew install cliproxyapi
brew services start cliproxyapi
```

### 通过 CLIProxyAPI Linux Installer 安装

```bash
curl -fsSL https://raw.githubusercontent.com/brokechubb/cliproxyapi-installer/refs/heads/master/cliproxyapi-installer | bash
```

感谢 [brokechubb](https://github.com/brokechubb) 构建了 Linux installer！

## 使用方法

### 图形客户端与官方 WebUI

#### [EasyCLI](https://github.com/router-for-me/EasyCLI)

CLIProxyAPI 的跨平台桌面图形客户端。

#### [Cli-Proxy-API-Management-Center](https://github.com/router-for-me/Cli-Proxy-API-Management-Center)

CLIProxyAPI 的基于 Web 的管理中心。

如果希望自行托管管理页面，可在配置中将 `remote-management.disable-control-panel` 设为 `true`，服务器将停止下载 `management.html`，并让 `/management.html` 返回 404。

可以通过设置环境变量 `MANAGEMENT_STATIC_PATH` 来指定 `management.html` 的存储目录。

### 身份验证

您可以分别为 Gemini、OpenAI、Claude、Qwen 和 iFlow 进行身份验证，它们可同时存在于同一个 `auth-dir` 中并参与负载均衡。

- Gemini（Google）：
  ```bash
  ./cli-proxy-api --login
  ```
  如果您是现有的 Gemini Code 用户，可能需要指定一个项目ID：
  ```bash
  ./cli-proxy-api --login --project_id <your_project_id>
  ```
  本地 OAuth 回调端口为 `8085`。

  选项：加上 `--no-browser` 可打印登录地址而不自动打开浏览器。本地 OAuth 回调端口为 `8085`。

- OpenAI（Codex/GPT，OAuth）：
  ```bash
  ./cli-proxy-api --codex-login
  ```
  选项：加上 `--no-browser` 可打印登录地址而不自动打开浏览器。本地 OAuth 回调端口为 `1455`。

- Claude（Anthropic，OAuth）：
  ```bash
  ./cli-proxy-api --claude-login
  ```
  选项：加上 `--no-browser` 可打印登录地址而不自动打开浏览器。本地 OAuth 回调端口为 `54545`。

- Qwen（Qwen Chat，OAuth）：
  ```bash
  ./cli-proxy-api --qwen-login
  ```
  选项：加上 `--no-browser` 可打印登录地址而不自动打开浏览器。使用 Qwen Chat 的 OAuth 设备登录流程。

- iFlow（iFlow，OAuth）：
  ```bash
  ./cli-proxy-api --iflow-login
  ```
  选项：加上 `--no-browser` 可打印登录地址而不自动打开浏览器。本地 OAuth 回调端口为 `11451`。

- Kiro（Kiro AI，基于令牌认证）：
  1. 从官方 Kiro 工具下载 `kiro-auth-token.json`。
  2. 确认 JSON 中包含 `"type": "kiro"` 字段（通过 CLI 导入器生成的文件会自动包含该字段；如果是手动提供的令牌，需要自行补全）。
  3. 将其放置到配置的 `auth-dir`（默认 `~/.cli-proxy-api/kiro-auth-token.json`）。
  3. 使用仓库提供的测试配置快速验证：
     ```bash
     ./cli-proxy-api --config config.test.yaml
     curl -H "Authorization: Bearer test-api-key-01" -H "Content-Type: application/json" -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}]}' http://localhost:8317/v1/chat/completions
     ```
     只要 `kiro-auth-token.json` 在 `auth-dir` 中，新 Kiro 执行器就会自动生效。

### 启动服务器

身份验证完成后，启动服务器：

```bash
./cli-proxy-api
```

默认情况下，服务器在端口 8317 上运行。

如需快速本地自测（尤其是 Kiro），可直接使用 `config.test.yaml`：

```bash
./cli-proxy-api --config config.test.yaml
```

该配置开启详细日志、使用伪造 API Key（`test-api-key-01`），方便用 `curl` 直接命中 Kiro/Gemini 端点。

### API 端点

#### 列出模型

```
GET http://localhost:8317/v1/models
```

#### 聊天补全

```
POST http://localhost:8317/v1/chat/completions
```

请求体示例：

```json
{
  "model": "gemini-2.5-pro",
  "messages": [
    {
      "role": "user",
      "content": "你好，你好吗？"
    }
  ],
  "stream": true
}
```

说明：
- 使用 "gemini-*" 模型（例如 "gemini-2.5-pro"）来调用 Gemini，使用 "gpt-*" 模型（例如 "gpt-5"）来调用 OpenAI，使用 "claude-*" 模型（例如 "claude-3-5-sonnet-20241022"）来调用 Claude，使用 "qwen-*" 模型（例如 "qwen3-coder-plus"）来调用 Qwen，使用 Kiro 模型（例如 "claude-sonnet-4-5"）来调用 Kiro AI，或者使用 iFlow 支持的模型（例如 "tstars2.0"、"deepseek-v3.1"、"kimi-k2" 等）来调用 iFlow。代理服务会自动将请求路由到相应的提供商。

#### Claude 消息（SSE 兼容）

```
POST http://localhost:8317/v1/messages
```

## CI / Docker 镜像

仓库内的 `.github/workflows/docker-build.yml` 定义了镜像构建流程：当推送到 `main` 或标签 `v*.*.*` 时，会以 Buildx 同时构建 `linux/amd64` 与 `linux/arm64` 镜像，并推送到 Docker Hub（`$DOCKERHUB_USERNAME/cliproxyapi`）和 GitHub Container Registry（`ghcr.io/<owner>/cliproxyapi`）。请在仓库 Secrets 中配置：

- `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN`
- （可选）若不想使用默认 `GITHUB_TOKEN`，可提供 `GHCR_PAT`

也可以在 Actions 页面使用 `workflow_dispatch` 手动触发该工作流。

### 与 OpenAI 库一起使用

您可以通过将基础 URL 设置为本地服务器来将此代理与任何 OpenAI 兼容的库一起使用：

#### Python（使用 OpenAI 库）

```python
from openai import OpenAI

client = OpenAI(
    api_key="dummy",  # 不使用但必需
    base_url="http://localhost:8317/v1"
)

# Gemini 示例
gemini = client.chat.completions.create(
    model="gemini-2.5-pro",
    messages=[{"role": "user", "content": "你好，你好吗？"}]
)

# Codex/GPT 示例
gpt = client.chat.completions.create(
    model="gpt-5",
    messages=[{"role": "user", "content": "用一句话总结这个项目"}]
)

# Claude 示例（使用 messages 端点）
import requests
claude_response = requests.post(
    "http://localhost:8317/v1/messages",
    json={
        "model": "claude-3-5-sonnet-20241022",
        "messages": [{"role": "user", "content": "用一句话总结这个项目"}],
        "max_tokens": 1000
    }
)

print(gemini.choices[0].message.content)
print(gpt.choices[0].message.content)
print(claude_response.json())
```

#### JavaScript/TypeScript

```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'dummy', // 不使用但必需
  baseURL: 'http://localhost:8317/v1',
});

// Gemini
const gemini = await openai.chat.completions.create({
  model: 'gemini-2.5-pro',
  messages: [{ role: 'user', content: '你好，你好吗？' }],
});

// Codex/GPT
const gpt = await openai.chat.completions.create({
  model: 'gpt-5',
  messages: [{ role: 'user', content: '用一句话总结这个项目' }],
});

// Claude 示例（使用 messages 端点）
const claudeResponse = await fetch('http://localhost:8317/v1/messages', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    model: 'claude-3-5-sonnet-20241022',
    messages: [{ role: 'user', content: '用一句话总结这个项目' }],
    max_tokens: 1000
  })
});

console.log(gemini.choices[0].message.content);
console.log(gpt.choices[0].message.content);
console.log(await claudeResponse.json());
```

## 支持的模型

- gemini-2.5-pro
- gemini-2.5-flash
- gemini-2.5-flash-lite
- gemini-2.5-flash-image
- gemini-2.5-flash-image-preview
- gemini-pro-latest
- gemini-flash-latest
- gemini-flash-lite-latest
- gpt-5
- gpt-5-codex
- claude-opus-4-1-20250805
- claude-opus-4-20250514
- claude-sonnet-4-20250514
- claude-sonnet-4-5-20250929
- claude-haiku-4-5-20251001
- claude-3-7-sonnet-20250219
- claude-3-5-haiku-20241022
- qwen3-coder-plus
- qwen3-coder-flash
- qwen3-max
- qwen3-vl-plus
- deepseek-v3.2
- deepseek-v3.1
- deepseek-r1
- deepseek-v3
- kimi-k2
- glm-4.6
- tstars2.0
- 以及其他 iFlow 支持的模型
- **Kiro AI 模型**：
  - claude-sonnet-4-5
  - claude-sonnet-4-5-20250929
  - claude-sonnet-4-20250514
  - claude-3-7-sonnet-20250219
  - amazonq-claude-sonnet-4-20250514
  - amazonq-claude-3-7-sonnet-20250219
- Gemini 模型在需要时自动切换到对应的 preview 版本

## 配置

服务器默认使用位于项目根目录的 YAML 配置文件（`config.yaml`）。您可以使用 `--config` 标志指定不同的配置文件路径：

```bash
  ./cli-proxy-api --config /path/to/your/config.yaml
```

### 配置选项

| 参数                                      | 类型       | 默认值                | 描述                                                                  |
|-----------------------------------------|----------|--------------------|---------------------------------------------------------------------|
| `port`                                  | integer  | 8317               | 服务器将监听的端口号。                                                         |
| `auth-dir`                              | string   | "~/.cli-proxy-api" | 存储身份验证令牌的目录。支持使用 `~` 来表示主目录。如果你使用Windows，建议设置成`C:/cli-proxy-api/`。  |
| `proxy-url`                             | string   | ""                 | 代理URL。支持socks5/http/https协议。例如：socks5://user:pass@192.168.1.1:1080/ |
| `request-retry`                         | integer  | 0                  | 请求重试次数。如果HTTP响应码为403、408、500、502、503或504，将会触发重试。                    |
| `remote-management.allow-remote`        | boolean  | false              | 是否允许远程（非localhost）访问管理接口。为false时仅允许本地访问；本地访问同样需要管理密钥。               |
| `remote-management.secret-key`          | string   | ""                 | 管理密钥。若配置为明文，启动时会自动进行bcrypt加密并写回配置文件。若为空，管理接口整体不可用（404）。             |
| `remote-management.disable-control-panel` | boolean  | false              | 当为 true 时，不再下载 `management.html`，且 `/management.html` 会返回 404，从而禁用内置管理界面。             |
| `quota-exceeded`                        | object   | {}                 | 用于处理配额超限的配置。                                                        |
| `quota-exceeded.switch-project`         | boolean  | true               | 当配额超限时，是否自动切换到另一个项目。                                                |
| `quota-exceeded.switch-preview-model`   | boolean  | true               | 当配额超限时，是否自动切换到预览模型。                                                 |
| `debug`                                 | boolean  | false              | 启用调试模式以获取详细日志。                                                      |
| `logging-to-file`                       | boolean  | true               | 是否将应用日志写入滚动文件；设为 false 时输出到 stdout/stderr。                           |
| `usage-statistics-enabled`              | boolean  | true               | 是否启用内存中的使用统计；设为 false 时直接丢弃所有统计数据。                               |
| `api-keys`                              | string[] | []                 | 兼容旧配置的简写，会自动同步到默认 `config-api-key` 提供方。                     |
| `gemini-api-key`                        | object[] | []                 | Gemini API 密钥配置，支持为每个密钥设置可选的 `base-url` 与 `proxy-url`。         |
| `gemini-api-key.*.api-key`              | string   | ""                 | Gemini API 密钥。                                                              |
| `gemini-api-key.*.base-url`             | string   | ""                 | 可选的 Gemini API 端点覆盖地址。                                              |
| `gemini-api-key.*.headers`              | object   | {}                 | 可选的额外 HTTP 头部，仅在访问覆盖后的 Gemini 端点时发送。                     |
| `gemini-api-key.*.proxy-url`            | string   | ""                 | 可选的单独代理设置，会覆盖全局 `proxy-url`。                                   |
| `generative-language-api-key`           | string[] | []                 | （兼容别名）旧管理接口返回的纯密钥列表。通过该接口写入会更新 `gemini-api-key`。 |
| `codex-api-key`                                       | object   | {}                 | Codex API密钥列表。                                                      |
| `codex-api-key.api-key`                               | string   | ""                 | Codex API密钥。                                                        |
| `codex-api-key.base-url`                              | string   | ""                 | 自定义的Codex API端点                                                     |
| `codex-api-key.proxy-url`                             | string   | ""                 | 针对该API密钥的代理URL。会覆盖全局proxy-url设置。支持socks5/http/https协议。                 |
| `claude-api-key`                                      | object   | {}                 | Claude API密钥列表。                                                     |
| `claude-api-key.api-key`                              | string   | ""                 | Claude API密钥。                                                       |
| `claude-api-key.base-url`                             | string   | ""                 | 自定义的Claude API端点，如果您使用第三方的API端点。                                    |
| `claude-api-key.proxy-url`                            | string   | ""                 | 针对该API密钥的代理URL。会覆盖全局proxy-url设置。支持socks5/http/https协议。                 |
| `claude-api-key.models`                               | object[] | []                 | Model alias entries for this key.                                      |
| `claude-api-key.models.*.name`                        | string   | ""                 | Upstream Claude model name invoked against the API.                    |
| `claude-api-key.models.*.alias`                       | string   | ""                 | Client-facing alias that maps to the upstream model name.              |
| `openai-compatibility`                                | object[] | []                 | 上游OpenAI兼容提供商的配置（名称、基础URL、API密钥、模型）。                                |
| `openai-compatibility.*.name`                         | string   | ""                 | 提供商的名称。它将被用于用户代理（User Agent）和其他地方。                                  |
| `openai-compatibility.*.base-url`                     | string   | ""                 | 提供商的基础URL。                                                          |
| `openai-compatibility.*.api-keys`                     | string[] | []                 | (已弃用) 提供商的API密钥。建议改用api-key-entries以获得每密钥代理支持。                       |
| `openai-compatibility.*.api-key-entries`              | object[] | []                 | API密钥条目，支持可选的每密钥代理配置。优先于api-keys。                                   |
| `openai-compatibility.*.api-key-entries.*.api-key`    | string   | ""                 | 该条目的API密钥。                                                          |
| `openai-compatibility.*.api-key-entries.*.proxy-url`  | string   | ""                 | 针对该API密钥的代理URL。会覆盖全局proxy-url设置。支持socks5/http/https协议。                 |
| `openai-compatibility.*.models`                       | object[] | []                 | Model alias definitions routing client aliases to upstream names.      |
| `openai-compatibility.*.models.*.name`                | string   | ""                 | Upstream model name invoked against the provider.                      |
| `openai-compatibility.*.models.*.alias`               | string   | ""                 | Client alias routed to the upstream model.                             |

When `claude-api-key.models` is provided, only the listed aliases are registered for that credential, and the default Claude model catalog is skipped.

### 配置文件示例

```yaml
# 服务器端口
port: 8317

# 管理 API 设置
remote-management:
  # 是否允许远程（非localhost）访问管理接口。为false时仅允许本地访问（但本地访问同样需要管理密钥）。
  allow-remote: false

  # 管理密钥。若配置为明文，启动时会自动进行bcrypt加密并写回配置文件。
  # 所有管理请求（包括本地）都需要该密钥。
  # 若为空，/v0/management 整体处于 404（禁用）。
  secret-key: ""

  # 当设为 true 时，不下载管理面板文件，/management.html 将直接返回 404。
  disable-control-panel: false

# 身份验证目录（支持 ~ 表示主目录）。如果你使用Windows，建议设置成`C:/cli-proxy-api/`。
auth-dir: "~/.cli-proxy-api"

# 请求认证使用的API密钥
api-keys:
  - "your-api-key-1"
  - "your-api-key-2"

# 启用调试日志
debug: false

# 为 true 时将应用日志写入滚动文件而不是 stdout
logging-to-file: true

# 为 false 时禁用内存中的使用统计并直接丢弃所有数据
usage-statistics-enabled: true

# 代理URL。支持socks5/http/https协议。例如：socks5://user:pass@192.168.1.1:1080/
proxy-url: ""

# 请求重试次数。如果HTTP响应码为403、408、500、502、503或504，将会触发重试。
request-retry: 3


# 配额超限行为
quota-exceeded:
   switch-project: true # 当配额超限时是否自动切换到另一个项目
   switch-preview-model: true # 当配额超限时是否自动切换到预览模型

# Gemini API 密钥
gemini-api-key:
  - api-key: "AIzaSy...01"
    base-url: "https://generativelanguage.googleapis.com"
    headers:
      X-Custom-Header: "custom-value"
    proxy-url: "socks5://proxy.example.com:1080"
  - api-key: "AIzaSy...02"

# Codex API 密钥
codex-api-key:
  - api-key: "sk-atSM..."
    base-url: "https://www.example.com" # 第三方 Codex API 中转服务端点
    proxy-url: "socks5://proxy.example.com:1080" # 可选:针对该密钥的代理设置

CLIProxyAPI 用户手册： [https://help.router-for.me/](https://help.router-for.me/cn/)

## 管理 API 文档

请参见 [MANAGEMENT_API_CN.md](https://help.router-for.me/cn/management/api)

## SDK 文档

- 使用文档：[docs/sdk-usage_CN.md](docs/sdk-usage_CN.md)
- 高级（执行器与翻译器）：[docs/sdk-advanced_CN.md](docs/sdk-advanced_CN.md)
- 认证: [docs/sdk-access_CN.md](docs/sdk-access_CN.md)
- 凭据加载/更新: [docs/sdk-watcher_CN.md](docs/sdk-watcher_CN.md)
- 自定义 Provider 示例：`examples/custom-provider`

## 贡献

欢迎贡献！请随时提交 Pull Request。

1. Fork 仓库
2. 创建您的功能分支（`git checkout -b feature/amazing-feature`）
3. 提交您的更改（`git commit -m 'Add some amazing feature'`）
4. 推送到分支（`git push origin feature/amazing-feature`）
5. 打开 Pull Request

## 谁与我们在一起？

这些项目基于 CLIProxyAPI:

### [vibeproxy](https://github.com/automazeio/vibeproxy)

一个原生 macOS 菜单栏应用，让您可以使用 Claude Code & ChatGPT 订阅服务和 AI 编程工具，无需 API 密钥。

### [Subtitle Translator](https://github.com/VjayC/SRT-Subtitle-Translator-Validator)

一款基于浏览器的 SRT 字幕翻译工具，可通过 CLI 代理 API 使用您的 Gemini 订阅。内置自动验证与错误修正功能，无需 API 密钥。

> [!NOTE]  
> 如果你开发了基于 CLIProxyAPI 的项目，请提交一个 PR（拉取请求）将其添加到此列表中。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。

## 写给所有中国网友的

QQ 群：188637136

或

Telegram 群：https://t.me/CLIProxyAPI
