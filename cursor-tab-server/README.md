### 如何获取 token？
**macos**
```bash
sqlite3 "$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb" \
  "SELECT key, value FROM ItemTable WHERE key IN ('cursorAuth/accessToken', 'cursorAuth/refreshToken');"

```
**windows 获取方式**
```bash
sqlite3 "$env:APPDATA\Cursor\User\globalStorage\state.vscdb" "SELECT key, value FROM ItemTable WHERE key IN ('cursorAuth/accessToken', 'cursorAuth/refreshToken');"


```

> 推荐直接在助手主配置 `~/.cursor-local-assistant-v2/config.yaml` 的 `cursor.accessToken` / `cursor.refreshToken` 填写，无需再运行本 tab-server。