# Windows 10 部署指南

目标：在一台 **Windows 10 x64** 上运行 AI Gateway，向局域网分发 OpenAI 兼容的模型 API Key。

不需要 Docker、Go、Python 或 Node。`ai-gateway.exe` 是自包含的单文件程序；Python/Node 只在“用客户端调用模型”时可选使用。

---

## 一、准备

1. 取得 `ai-gateway_1.0.0_windows_amd64.zip`；
2. 解压到 **`C:\ai-gateway`**（建议路径简短、无中文；其他路径也可以）；
3. 确认解压后有：

```text
C:\ai-gateway\
├── ai-gateway.exe
├── install-windows.ps1
├── uninstall-windows.ps1
├── config.example.json
├── run-windows.bat
└── README.md
```

4. 用管理员身份打开 PowerShell：开始菜单 → 搜索 `PowerShell` → 右键 → **以管理员身份运行**。

> Windows 10 的 PowerShell 5.1 默认禁止运行脚本。下面命令用 `-ExecutionPolicy Bypass` 只对本次安装放行，不修改系统策略。

---

## 二、安装

```powershell
cd C:\ai-gateway
powershell -ExecutionPolicy Bypass -File .\install-windows.ps1
```

默认参数：

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-InstallDir` | `C:\ai-gateway` | 程序目录 |
| `-DataDir` | `C:\ProgramData\ai-gateway\data` | 数据库、主密钥、首次密码文件 |
| `-Port` | `8080` | 监听端口 |
| `-SiteName` | `AI 模型中转站` | 页面显示名称 |
| `-TaskName` | `AI Gateway` | 计划任务名称 |
| `-NoFirewall` | 不加为自动 | 跳过防火墙规则 |

示例：

```powershell
powershell -ExecutionPolicy Bypass -File .\install-windows.ps1 `
  -Port 9000 -SiteName "公司模型网关"
```

安装脚本会：

- 复制 `ai-gateway.exe` 到安装目录；
- 创建 `C:\ProgramData\ai-gateway\config.json`（已存在则保留）和 `logs`、`data` 目录；
- 把数据目录 ACL 收紧为仅 `SYSTEM` 和 `Administrators` 可访问；
- 注册**开机自启计划任务** `AI Gateway`，以 `SYSTEM` 身份后台运行（无需登录桌面）；
- 放行 Windows 防火墙 TCP 端口（所有网络配置文件）；
- 启动服务并做健康检查，最后打印地址和首次登录密码。

重复运行同一个脚本即为**升级**：会停旧服务、替换 exe，保留配置、数据库、主密钥和日志。

### 安装失败时

- 确认 PowerShell 是“管理员”窗口；
- 确认 zip 已完整解压，且当前目录有 `ai-gateway.exe`；
- 若提示端口占用：

```powershell
netstat -ano | findstr :8080
Get-Process -Id <PID>
```

换端口重新安装即可：

```powershell
powershell -ExecutionPolicy Bypass -File .\install-windows.ps1 -Port 9000
```

- 若 Windows Defender/SmartScreen 拦截未签名程序，请核对文件来源和 SHA256 后选择允许，不要把整个目录加入无差别排除。

---

## 三、首次登录

```powershell
Get-Content C:\ProgramData\ai-gateway\data\initial-admin.txt
```

浏览器打开：

```text
http://127.0.0.1:8080
http://<这台电脑的局域网IP>:8080
```

用文件中的用户名/密码登录，进入“系统设置”修改密码，然后删除：

```powershell
Remove-Item C:\ProgramData\ai-gateway\data\initial-admin.txt
```

获取本机局域网 IP：

```powershell
Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object { $_.IPAddress -notlike "127.*" } |
  Select-Object IPAddress, InterfaceAlias
```

---

## 四、配置模型

登录后按以下顺序操作（详细字段说明见 `README.md` 第四节）：

1. **供应商**：添加上游，例如 vLLM `http://10.0.0.21:8000`、Ollama `http://10.0.0.22:11434`；
2. 点“测试连接”，再点“同步模型”；
3. **模型与路由**：创建对外别名（如 `qwen2.5-7b`），添加上游路由，设置优先级/权重；
4. **用户管理**：创建成员账号，设置可用模型和账号级总额度；
5. **API 密钥**：创建 Key 并复制明文（只显示一次）；
6. 用下面的 Python/Node 或任意 OpenAI SDK 调用验证。

---

## 五、服务管理

```powershell
# 查看状态和上次运行结果
Get-ScheduledTask -TaskName "AI Gateway"
Get-ScheduledTaskInfo -TaskName "AI Gateway"

# 停止 / 启动
Stop-ScheduledTask  -TaskName "AI Gateway"
Start-ScheduledTask -TaskName "AI Gateway"

# 实时日志
Get-Content "C:\ProgramData\ai-gateway\logs\gateway.log" -Tail 50 -Wait
```

计划任务设置为开机启动、失败自动重试 3 次（间隔 1 分钟），不会因为注销当前用户而停止。

---

## 六、日志与排查

| 项目 | 位置/命令 |
|---|---|
| 运行日志 | `C:\ProgramData\ai-gateway\logs\gateway.log`（超过 20 MB 启动时轮换为 `.1`） |
| 配置文件 | `C:\ProgramData\ai-gateway\config.json` |
| 数据库 | `C:\ProgramData\ai-gateway\data\gateway.db` |
| 主密钥 | `C:\ProgramData\ai-gateway\data\master.key` |
| 健康检查 | `Invoke-WebRequest http://127.0.0.1:8080/healthz` |
| 端口占用 | `netstat -ano \| findstr :8080` |
| 防火墙规则 | `Get-NetFirewallRule -DisplayName "AI Gateway"` |

常见问题：

- **本机能开、别的电脑打不开**：检查防火墙规则、Windows 网络位置（安装脚本已允许所有配置文件）、交换机/ACL；
- **页面打开但登录 401**：确认用的是网关分发的 `gw-...` Key，而不是上游供应商的 Key；
- **调用返回 502/503**：到“供应商”点“测试连接”，检查 Base URL、上游服务、内网连通性；
- **更改端口后仍访问旧端口**：安装脚本会保留已有 `config.json`，请编辑配置中的 `listen`，或用 `-Port` 删除旧配置后重装；
- **杀毒软件拦截**：确认哈希后放行 `ai-gateway.exe`。

---

## 七、升级

1. 把新版本 zip 解压到新目录（例如 `C:\ai-gateway-1.0.1`）；
2. 管理员 PowerShell 执行：

```powershell
cd C:\ai-gateway-1.0.1
powershell -ExecutionPolicy Bypass -File .\install-windows.ps1
```

安装脚本会停止旧任务、替换 `C:\ai-gateway\ai-gateway.exe`，保留：

```text
C:\ProgramData\ai-gateway\config.json
C:\ProgramData\ai-gateway\data\gateway.db
C:\ProgramData\ai-gateway\data\master.key
```

升级前建议先做一次备份（见下一节）。

---

## 八、备份与恢复

数据库与 `master.key` 必须一起备份，否则供应商密钥无法解密。

### 备份

```powershell
Stop-ScheduledTask -TaskName "AI Gateway"
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
Compress-Archive -Path "C:\ProgramData\ai-gateway\*" `
  -DestinationPath "D:\backup\ai-gateway-$stamp.zip"
Start-ScheduledTask -TaskName "AI Gateway"
```

建议把备份复制到另一台机器或移动硬盘。

### 恢复

```powershell
Stop-ScheduledTask -TaskName "AI Gateway"
# 先备份当前目录，再覆盖
Compress-Archive -Path "C:\ProgramData\ai-gateway\*" `
  -DestinationPath "D:\backup\before-restore.zip" -Force
Remove-Item "C:\ProgramData\ai-gateway\data" -Recurse -Force
Expand-Archive -Path "D:\backup\ai-gateway-20261115-120000.zip" `
  -DestinationPath "C:\ProgramData\ai-gateway" -Force
& icacls.exe "C:\ProgramData\ai-gateway\data" /inheritance:r `
  /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" | Out-Null
Start-ScheduledTask -TaskName "AI Gateway"
```

恢复后确认数据目录 ACL 仅 SYSTEM/Administrators 可访问：

```powershell
icacls "C:\ProgramData\ai-gateway\data"
```

---

## 九、忘记管理员密码

在服务器本机、管理员 PowerShell 中执行：

```powershell
Stop-ScheduledTask -TaskName "AI Gateway"

$env:GATEWAY_RESET_PASSWORD = "至少8位的新密码"
& "C:\ai-gateway\ai-gateway.exe" -config "C:\ProgramData\ai-gateway\config.json" `
  -reset-password-for admin
Remove-Item Env:\GATEWAY_RESET_PASSWORD

Start-ScheduledTask -TaskName "AI Gateway"
```

该账号的所有登录会话会立即失效。

---

## 十、卸载

```powershell
cd C:\ai-gateway
powershell -ExecutionPolicy Bypass -File .\uninstall-windows.ps1
```

默认保留 `C:\ProgramData\ai-gateway` 中的数据。确认不再需要后彻底删除：

```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall-windows.ps1 -RemoveData
```

`-RemoveData` 会删除数据库和 `master.key`，不可恢复。

---

## 十一、用 Python / Node 调用（可选）

Python 和 Node 只是客户端，不参与网关运行。以下示例假设网关地址为 `http://127.0.0.1:8080`，Key 为管理员分发的 `gw-...`。

### Python（标准库，无需安装依赖）

```python
import json
import urllib.request

BASE = "http://127.0.0.1:8080"
KEY = "gw-xxxxxxxx"

print(urllib.request.urlopen(BASE + "/healthz").read().decode())

body = json.dumps({
    "model": "qwen2.5-7b",
    "messages": [{"role": "user", "content": "你好"}],
}).encode()
req = urllib.request.Request(
    BASE + "/v1/chat/completions",
    data=body,
    headers={
        "Authorization": "Bearer " + KEY,
        "Content-Type": "application/json",
    },
)
print(urllib.request.urlopen(req).read().decode())
```

如果已安装 `openai` 包：

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:8080/v1", api_key="gw-xxxxxxxx")
print(client.chat.completions.create(
    model="qwen2.5-7b",
    messages=[{"role": "user", "content": "你好"}],
).choices[0].message.content)
```

### Node.js（18+ 自带 fetch）

```js
const res = await fetch("http://127.0.0.1:8080/v1/chat/completions", {
  method: "POST",
  headers: {
    Authorization: "Bearer gw-xxxxxxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "qwen2.5-7b",
    messages: [{ role: "user", content: "你好" }],
  }),
});
console.log(await res.text());
```

其他客户端（Cherry Studio、Chatbox、Open WebUI、Dify、LangChain 等）填写：

```text
Base URL: http://<服务器IP>:8080/v1
API Key : gw-xxxxxxxx
Model   : 网关中配置的模型别名
```

---

## 十二、安全建议

- 安装后立即修改初始密码并删除 `initial-admin.txt`；
- 计划任务以 `SYSTEM` 运行，数据目录已限制 ACL；不要把数据目录共享给普通用户；
- 仅在内网开放端口；如条件允许，用防火墙限制来源网段；
- 定期备份 `C:\ProgramData\ai-gateway`，备份中包含主密钥，按敏感文件保管；
- 为不同应用创建不同 Key，设置用户级总额度和 Key 级限额；
- 关注“审计日志”和“调用日志”，异常 Key 立即禁用或轮换。
