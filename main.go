package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ai-gateway/internal/gateway"
	"ai-gateway/internal/httpapi"
	"ai-gateway/internal/limiter"
	"ai-gateway/internal/secure"
	"ai-gateway/internal/store"
)

var version = "dev"

type fileConfig struct {
	Listen          string `json:"listen"`
	DataDir         string `json:"data_dir"`
	SiteName        string `json:"site_name"`
	TrustProxy      bool   `json:"trust_proxy"`
	SessionTTLHours int    `json:"session_ttl_hours"`
	LogFile         string `json:"log_file"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath  = flag.String("config", "", "配置文件路径 (JSON)")
		listen      = flag.String("listen", "", "监听地址，例如 :8080")
		dataDir     = flag.String("data", "", "数据目录")
		logFile     = flag.String("log", "", "日志文件路径（默认输出到标准输出）")
		resetUser   = flag.String("reset-password-for", "", "重置指定用户密码后退出（密码从环境变量 GATEWAY_RESET_PASSWORD 读取）")
		showVersion = flag.Bool("version", false, "打印版本后退出")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println("ai-gateway", version)
		return nil
	}

	cfg := fileConfig{
		Listen:          ":8080",
		DataDir:         "./data",
		SiteName:        "AI 模型中转站",
		SessionTTLHours: 168,
	}
	if *configPath != "" {
		b, err := os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("读取配置文件: %w", err)
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return fmt.Errorf("解析配置文件: %w", err)
		}
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *logFile != "" {
		cfg.LogFile = *logFile
	}
	if v := os.Getenv("GATEWAY_LOG_FILE"); v != "" {
		cfg.LogFile = v
	}
	if v := os.Getenv("GATEWAY_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("GATEWAY_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("GATEWAY_SITE_NAME"); v != "" {
		cfg.SiteName = v
	}
	if v := os.Getenv("GATEWAY_TRUST_PROXY"); v != "" {
		cfg.TrustProxy = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("GATEWAY_SESSION_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SessionTTLHours = n
		}
	}

	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("GATEWAY_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logOutput := io.Writer(os.Stdout)
	if strings.TrimSpace(cfg.LogFile) != "" {
		absLog, err := filepath.Abs(cfg.LogFile)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(absLog), 0o750); err != nil {
			return fmt.Errorf("创建日志目录: %w", err)
		}
		// Keep the log from growing without bound; one rotated file is enough
		// for this deployment size.
		if fi, err := os.Stat(absLog); err == nil && fi.Size() > 20<<20 {
			_ = os.Remove(absLog + ".1")
			_ = os.Rename(absLog, absLog+".1")
		}
		f, err := os.OpenFile(absLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return fmt.Errorf("打开日志文件: %w", err)
		}
		defer f.Close()
		logOutput = io.MultiWriter(os.Stdout, f)
	}
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	absData, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absData, 0o750); err != nil {
		return fmt.Errorf("创建数据目录: %w", err)
	}
	if err := os.Chmod(absData, 0o750); err != nil {
		logger.Warn("cannot chmod data dir", "err", err)
	}

	st, err := store.Open(filepath.Join(absData, "gateway.db"))
	if err != nil {
		return fmt.Errorf("打开数据库: %w", err)
	}
	defer st.Close()

	vault, err := secure.LoadOrCreateVault(filepath.Join(absData, "master.key"))
	if err != nil {
		return fmt.Errorf("初始化密钥保险箱: %w", err)
	}

	if *resetUser != "" {
		return resetPassword(st, *resetUser)
	}
	if err := bootstrapAdmin(st, absData, logger); err != nil {
		return err
	}
	if err := applyDefaultSettings(st, cfg); err != nil {
		logger.Warn("apply default settings", "err", err)
	}

	lim := limiter.New()
	gw := gateway.New(st, vault, lim, logger)
	srv := httpapi.New(st, vault, gw, lim, httpapi.Config{
		SiteName:   cfg.SiteName,
		Version:    version,
		DataDir:    absData,
		TrustProxy: cfg.TrustProxy,
		SessionTTL: time.Duration(cfg.SessionTTLHours) * time.Hour,
	}, logger)

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go backgroundMaintenance(ctx, st, lim, srv, logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("ai-gateway started",
			"listen", cfg.Listen, "data_dir", absData, "version", version,
			"dashboard", "http://"+displayAddr(cfg.Listen))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}

func displayAddr(listen string) string {
	if strings.HasPrefix(listen, ":") {
		return "127.0.0.1" + listen
	}
	if strings.HasPrefix(listen, "0.0.0.0:") {
		return "127.0.0.1:" + strings.TrimPrefix(listen, "0.0.0.0:")
	}
	return listen
}

// resetPassword is the offline break-glass path for a locked-out account. It
// must be run on the server itself with access to the data directory.
func resetPassword(st *store.Store, username string) error {
	password := os.Getenv("GATEWAY_RESET_PASSWORD")
	if len(password) < 8 {
		return errors.New("请通过环境变量 GATEWAY_RESET_PASSWORD 提供至少 8 位的新密码")
	}
	if strings.TrimSpace(username) == "" {
		return errors.New("请用 -reset-password-for 指定用户名")
	}
	u, err := st.UserByUsername(username)
	if err != nil {
		return fmt.Errorf("用户 %s 不存在", username)
	}
	hash, err := secure.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.UpdatePassword(u.ID, hash); err != nil {
		return err
	}
	if err := st.DeleteSessionsForUser(u.ID); err != nil {
		return err
	}
	fmt.Printf("已重置用户 %s 的密码，该用户的所有登录会话已失效。\n", username)
	return nil
}

func bootstrapAdmin(st *store.Store, dataDir string, logger *slog.Logger) error {
	n, err := st.CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	username := strings.TrimSpace(os.Getenv("GATEWAY_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("GATEWAY_ADMIN_PASSWORD")
	generated := false
	if password == "" {
		password = secure.RandomPassword(16)
		generated = true
	}
	hash, err := secure.HashPassword(password)
	if err != nil {
		return fmt.Errorf("管理员初始密码不合规: %w", err)
	}
	admin := &store.User{
		Username:      username,
		PasswordHash:  hash,
		DisplayName:   "系统管理员",
		Role:          "admin",
		Status:        "active",
		AllowedModels: []string{"*"},
		Note:          "首次启动自动创建",
	}
	if err := st.CreateUser(admin); err != nil {
		return fmt.Errorf("创建初始管理员: %w", err)
	}
	if generated {
		path := filepath.Join(dataDir, "initial-admin.txt")
		msg := fmt.Sprintf(`============================================================
  已创建初始管理员账号（仅显示这一次）
    用户名: %s
    密码:   %s
  请登录后立即修改密码，并删除该提示文件。
  该信息同时保存在: %s
============================================================
`, username, password, path)
		fmt.Print(msg)
		_ = os.WriteFile(path, []byte(fmt.Sprintf("username: %s\npassword: %s\n", username, password)), 0o600)
	} else {
		logger.Info("initial admin created from GATEWAY_ADMIN_PASSWORD", "username", username)
	}
	return nil
}

func applyDefaultSettings(st *store.Store, cfg fileConfig) error {
	if v, _ := st.GetSetting("site_name"); v == "" {
		return st.SetSetting("site_name", cfg.SiteName)
	}
	return nil
}

func backgroundMaintenance(ctx context.Context, st *store.Store, lim *limiter.Limiter,
	srv *httpapi.Server, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	lastRetention := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			lim.Cleanup()
			srv.LoginLimiter.Cleanup()
			_ = st.DeleteExpiredSessions(now.Unix())
			if now.Sub(lastRetention) >= time.Hour {
				lastRetention = now
				days := 90
				if v, _ := st.GetSetting("log_retention_days"); v != "" {
					if n, err := strconv.Atoi(v); err == nil && n > 0 {
						days = n
					}
				}
				cutoff := now.AddDate(0, 0, -days)
				if n, err := st.DeleteLogsBefore(cutoff.Unix()); err != nil {
					logger.Error("cleanup request logs", "err", err)
				} else if n > 0 {
					logger.Info("cleaned request logs", "rows", n, "retention_days", days)
				}
				usageCutoff := now.AddDate(0, -13, 0).Format("2006-01-02")
				if n, err := st.DeleteUsageBefore(usageCutoff); err != nil {
					logger.Error("cleanup usage", "err", err)
				} else if n > 0 {
					logger.Info("cleaned usage rows", "rows", n)
				}
			}
		}
	}
}
