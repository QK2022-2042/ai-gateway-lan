#!/usr/bin/env bash
# AI Gateway 离线安装脚本（Linux，无需 Docker）
#
# 用法：
#   sudo ./install.sh                     # 默认监听 8080，数据在 /var/lib/ai-gateway
#   sudo ./install.sh --port 9000         # 自定义端口
#   sudo ./install.sh --data-dir /data/ai-gateway --site-name "公司模型网关"
#   sudo ./install.sh --no-start          # 只安装，不立即启动
#
# 重复执行即为升级：会保留配置、数据库与密钥，仅替换二进制。
set -euo pipefail

PORT=8080
DATA_DIR=/var/lib/ai-gateway
SITE_NAME="AI 模型中转站"
START=1

BIN_PATH=/usr/local/bin/ai-gateway
CONF_DIR=/etc/ai-gateway
CONF_FILE=$CONF_DIR/config.json
UNIT_FILE=/etc/systemd/system/ai-gateway.service
RUN_USER=ai-gateway

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_BIN="$SCRIPT_DIR/ai-gateway"

die()  { echo "错误：$*" >&2; exit 1; }
info() { echo "==> $*"; }
warn() { echo "警告：$*" >&2; }

usage() {
  sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)       PORT="${2:?--port 需要一个端口号}"; shift 2 ;;
    --data-dir)   DATA_DIR="${2:?--data-dir 需要一个路径}"; shift 2 ;;
    --site-name)  SITE_NAME="${2:?--site-name 需要一个名称}"; shift 2 ;;
    --no-start)   START=0; shift ;;
    -h|--help)    usage ;;
    *)            die "未知参数：$1（使用 --help 查看用法）" ;;
  esac
done

[[ $EUID -eq 0 ]] || die "请使用 root 运行：sudo ./install.sh"
[[ -f "$SRC_BIN" ]] || die "当前目录缺少 ai-gateway 二进制文件，请在解压后的安装包目录中运行"
[[ "$PORT" =~ ^[0-9]+$ ]] && (( PORT > 0 && PORT < 65536 )) || die "端口无效：$PORT"
[[ "$DATA_DIR" != *" "* && "$DATA_DIR" != *'"'* && "$DATA_DIR" != *"\\"* ]] \
  || die "数据目录不能包含空格、双引号或反斜杠（systemd/JSON 限制）"
[[ "$SITE_NAME" != *'"'* && "$SITE_NAME" != *"\\"* && "$SITE_NAME" != *$'\n'* ]] \
  || die "站点名称不能包含双引号、反斜杠或换行"

DATA_DIR="${DATA_DIR%/}"
if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
  HAS_SYSTEMD=1
else
  HAS_SYSTEMD=0
fi

# ---- 架构校验，防止把 x86 包装到 ARM 机器上 ----
machine="$(uname -m)"
case "$machine" in
  x86_64|amd64)  host_arch=amd64 ;;
  aarch64|arm64) host_arch=arm64 ;;
  *)             die "暂不支持当前架构：$machine" ;;
esac
if command -v od >/dev/null 2>&1; then
  elf_machine="$(od -An -t x1 -j 18 -N 2 "$SRC_BIN" 2>/dev/null | tr -d ' \n')"
  case "$elf_machine" in
    3e00) bin_arch=amd64 ;;
    b700) bin_arch=arm64 ;;
    *)    bin_arch="" ;;
  esac
  if [[ -n "$bin_arch" && "$bin_arch" != "$host_arch" ]]; then
    die "安装包架构($bin_arch)与本机($host_arch)不一致，请换用对应的 tar.gz"
  fi
fi

# ---- 用户与目录 ----
if ! id "$RUN_USER" >/dev/null 2>&1; then
  info "创建系统用户 $RUN_USER"
  if ! getent group "$RUN_USER" >/dev/null 2>&1; then
    groupadd --system "$RUN_USER" || die "创建用户组失败"
  fi
  nologin=/usr/sbin/nologin
  [[ -x "$nologin" ]] || nologin=/sbin/nologin
  [[ -x "$nologin" ]] || nologin=/bin/false
  useradd --system --gid "$RUN_USER" --home-dir "$DATA_DIR" \
    --shell "$nologin" "$RUN_USER" || die "创建用户失败"
fi

info "准备目录"
install -d -m 0755 "$CONF_DIR"
install -d -m 0750 -o "$RUN_USER" -g "$RUN_USER" "$DATA_DIR"

# ---- 升级时先停服务，避免替换正在运行的二进制 ----
if [[ "$HAS_SYSTEMD" == "1" ]] && systemctl is-active --quiet ai-gateway 2>/dev/null; then
  info "停止旧服务"
  systemctl stop ai-gateway
fi

info "安装二进制到 $BIN_PATH"
install -m 0755 "$SRC_BIN" "$BIN_PATH.new"
mv -f "$BIN_PATH.new" "$BIN_PATH"

# ---- 配置文件：已存在则不覆盖 ----
if [[ -f "$CONF_FILE" ]]; then
  info "保留已有配置 $CONF_FILE"
else
  info "生成配置 $CONF_FILE"
  cat > "$CONF_FILE" <<EOF
{
  "listen": ":$PORT",
  "data_dir": "$DATA_DIR",
  "site_name": "$SITE_NAME",
  "trust_proxy": false,
  "session_ttl_hours": 168
}
EOF
  chmod 0640 "$CONF_FILE"
fi
chown root:"$RUN_USER" "$CONF_FILE" 2>/dev/null || true
chown -R "$RUN_USER:$RUN_USER" "$DATA_DIR"

# ---- systemd 服务 ----
if [[ "$HAS_SYSTEMD" == "1" ]]; then
  if [[ ! -f "$UNIT_FILE" ]] || ! grep -q "^ReadWritePaths=$DATA_DIR$" "$UNIT_FILE" 2>/dev/null; then
    info "写入 systemd 服务 $UNIT_FILE"
    cat > "$UNIT_FILE" <<EOF
[Unit]
Description=AI Gateway - OpenAI-compatible model API relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
ExecStart=$BIN_PATH -config $CONF_FILE
Restart=on-failure
RestartSec=3
Environment=GATEWAY_LOG_LEVEL=info

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
  fi
  systemctl enable ai-gateway >/dev/null 2>&1 || true

  if [[ "$START" == "1" ]]; then
    info "启动服务"
    if ! systemctl restart ai-gateway; then
      warn "服务启动失败，最近日志："
      journalctl -u ai-gateway -n 30 --no-pager 2>/dev/null || true
      exit 1
    fi
  fi
else
  warn "系统未使用 systemd，已跳过服务安装。可手动启动："
  echo "    sudo -u $RUN_USER $BIN_PATH -config $CONF_FILE"
fi

# ---- 等待健康检查并输出访问信息 ----
if [[ "$START" == "1" ]] && [[ "$HAS_SYSTEMD" == "1" ]]; then
  ready=0
  for _ in $(seq 1 30); do
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then ready=1; break; fi
    else
      if (echo >/dev/tcp/127.0.0.1/"$PORT") >/dev/null 2>&1; then ready=1; break; fi
    fi
    sleep 0.5
  done
  if [[ "$ready" == "1" ]]; then
    info "服务已启动"
  else
    warn "服务未在 15 秒内就绪，请执行：journalctl -u ai-gateway -n 50 --no-pager"
  fi
fi

server_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
[[ -n "$server_ip" ]] || server_ip="<服务器IP>"

echo
echo "============================================================"
echo "  安装完成"
echo "    管理界面 : http://$server_ip:$PORT"
echo "    配置文件 : $CONF_FILE"
echo "    数据目录 : $DATA_DIR"
if [[ "$HAS_SYSTEMD" == "1" ]]; then
  echo "    服务管理 : systemctl status|restart|stop ai-gateway"
  echo "    服务日志 : journalctl -u ai-gateway -f"
fi
echo
if [[ -f "$DATA_DIR/initial-admin.txt" ]]; then
  echo "  首次登录凭据（登录后请立即改密并删除该文件）："
  sed 's/^/    /' "$DATA_DIR/initial-admin.txt"
  echo "    删除命令 : sudo rm -f $DATA_DIR/initial-admin.txt"
elif [[ "$HAS_SYSTEMD" == "1" ]]; then
  echo "  数据目录中已有账号，初始密码请使用既有管理员账号。"
fi
echo
echo "  如启用了防火墙，放行端口："
echo "    firewalld: firewall-cmd --permanent --add-port=$PORT/tcp && firewall-cmd --reload"
echo "    ufw      : ufw allow $PORT/tcp"
echo "============================================================"
