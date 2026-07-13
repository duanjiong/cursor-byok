# 已安装客户端只读核对与验证

首要原则：

- 不要修改已安装的 Cursor 客户端代码、bundle、签名或 app 副本。
- 允许且推荐读取、搜索、比对和分析客户端 bundle、日志、端口和 history 状态。
- 目标是定位差异、收集证据、判断问题归属，而不是 patch 客户端。

**cursor-byok 默认 app：`/Applications/Cursor BYOK.app`**。下文 `BYOK_APP` 指该路径；user data 在 `~/Library/Application Support/Cursor-BYOK/`。

## 1. 先确认实际运行的 app 副本

优先用非交互命令核对：

```bash
pgrep -fal 'Cursor BYOK|Cursor Hooked|Cursor Patched|/Contents/MacOS/Cursor'
ps -axo pid,ppid,command | rg 'Cursor BYOK|Cursor(.app)?/Contents/MacOS/Cursor|extension-host'
```

本仓库联调默认 **Cursor BYOK.app**；不要未确认进程就改查 `/Applications/Cursor.app`。不要修改客户端文件。

## 2. 只读定位目标 bundle 与关键文件

优先定位并读取这些文件，而不是改写它们：

```bash
BYOK_APP="/Applications/Cursor BYOK.app/Contents/Resources/app"
ls -l "$BYOK_APP/extensions"
shasum -a 256 "$BYOK_APP/extensions/cursor-always-local/dist/main.js"
```

重点关注：

- `out/vs/workbench/workbench.desktop.main.js`
- `out/vs/workbench/api/node/extensionHostProcess.js`
- `cursor-always-local/dist/main.js`
- `cursor-always-local/dist/gitWorker.js`
- `cursor-agent-exec/dist/main.js`
- `cursor-agent-exec/dist/*.js`
- `cursor-agent-worker/dist/main.js`

当前安装包里旧路径 `cursor-agent/dist/main.js` 通常不存在；先确认 `extensions/` 里的实际扩展名和 `dist/` 文件，再选择 `workbench` / `cursor-agent-exec` / `cursor-agent-worker` / `cursor-always-local` 对应排查。

## 3. 只读读取 bundle 内容

常用定位关键词：

```bash
BYOK_APP="/Applications/Cursor BYOK.app/Contents/Resources/app"
rg -n 'BidiTransport|ExecClientMessage|InteractionResponse|conversation_checkpoint_update' "$BYOK_APP/extensions/cursor-always-local/dist/main.js"
rg -n 'CursorAgentProvider|AnthropicProxy|ANTHROPIC_BASE_URL|InteractionUpdate|checkpoint|agent window' "$BYOK_APP/extensions/cursor-agent-exec/dist/main.js" "$BYOK_APP/extensions/cursor-agent-exec/dist"/*.js "$BYOK_APP/extensions/cursor-agent-worker/dist/main.js"
rg -n 'agent window|open_agent_window|NameAgent|UpdateConversationMetadata|shouldShowAgentWindowTitleHelperText' "$BYOK_APP/out/vs/workbench/workbench.desktop.main.js" "$BYOK_APP/extensions/cursor-agent-exec/dist"/*.js
```

读取具体文件内容时，优先用读取工具按需查看相关片段，不要修改 bundle。

如果需要和仓库实现对照，优先同时打开：

- `proto/agent_v1.proto`
- `proto/aiserver_v1.proto`
- `internal/backend/...`
- `internal/runtime/local_runtime.go`

## 4. 验证行为是否命中目标副本

至少做其中两项：

- 进程路径是否是目标 app
- 目标扩展 host 是否起来
- 本地监听端口是否存在
- `~/.cursor-local-assistant-v2/logs/app.log` 是否更新
- `~/Library/Application Support/Cursor-BYOK/logs/<session>/window1/renderer.log` 是否有客户端错误
- `~/.cursor-local-assistant-v2/history/<conversationId>/state.json` / `context.json` 是否更新
- 请求/协议事件是否真的经过你正在分析的 bundle 文件

常用验证：

```bash
pgrep -fal '/Applications/Cursor BYOK.app/Contents/MacOS/Cursor'
lsof -nP -iTCP -sTCP:LISTEN | rg 'Cursor|127.0.0.1'
```

## 5. 记录证据并输出归因

如果确认“已安装 app 行为”和“仓库代码理解”存在差异，优先记录：

1. 实际运行的 app 路径
2. 命中的 bundle 文件路径与关键符号
3. 对应日志、端口、`history/state.json`、`history/context.json`、`usage.json` 证据
4. 仓库里对应实现的位置

如果结论指向客户端侧，也停留在分析和归因，不要继续 patch、重签名、替换文件或做写入式验证。
