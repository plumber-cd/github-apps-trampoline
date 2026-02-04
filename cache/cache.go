package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/plumber-cd/github-apps-trampoline/logger"
)

type Config struct {
	Enabled          bool
	Dir              string
	TTLInstallations time.Duration
	TTLOwnerMapping  time.Duration
	TTLToken         time.Duration
	LockTimeout      time.Duration
	LockPollInterval time.Duration
}

type entry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	FetchedAt time.Time       `json:"fetched_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

var cfg Config

func Configure(config Config) {
	cfg = config
	if cfg.Dir == "" {
		cfg.Dir = defaultCacheDir()
	}
	if cfg.TTLInstallations == 0 {
		cfg.TTLInstallations = 5 * time.Minute
	}
	if cfg.TTLOwnerMapping == 0 {
		cfg.TTLOwnerMapping = 5 * time.Minute
	}
	if cfg.TTLToken == 0 {
		cfg.TTLToken = 10 * time.Minute
	}
	if cfg.LockTimeout == 0 {
		cfg.LockTimeout = 30 * time.Second
	}
	if cfg.LockPollInterval == 0 {
		cfg.LockPollInterval = 200 * time.Millisecond
	}
}

func Enabled() bool {
	return cfg.Enabled
}

func TTLInstallations() time.Duration {
	return cfg.TTLInstallations
}

func TTLOwnerMapping() time.Duration {
	return cfg.TTLOwnerMapping
}

func TTLToken() time.Duration {
	return cfg.TTLToken
}

func Get(key string, dest interface{}) (bool, error) {
	if !Enabled() {
		return false, nil
	}
	cachePath, keyHash, err := cachePathForKey(key)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logEvent("miss", key, keyHash, "not_found")
			return false, nil
		}
		return false, err
	}
	entryData := entry{}
	if err := json.Unmarshal(data, &entryData); err != nil {
		logEvent("miss", key, keyHash, "corrupt")
		return false, nil
	}
	if entryData.Key != "" && entryData.Key != key {
		logEvent("miss", key, keyHash, "key_mismatch")
		return false, nil
	}
	if time.Now().After(entryData.ExpiresAt) {
		logEvent("miss", key, keyHash, "expired")
		return false, nil
	}
	if err := json.Unmarshal(entryData.Value, dest); err != nil {
		logEvent("miss", key, keyHash, "unmarshal_failed")
		return false, nil
	}
	logEvent("hit", key, keyHash, "")
	return true, nil
}

func Set(key string, value interface{}, ttl time.Duration) error {
	if !Enabled() {
		return nil
	}
	cachePath, keyHash, err := cachePathForKey(key)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	entryData := entry{
		Key:       key,
		Value:     raw,
		FetchedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	payload, err := json.MarshalIndent(entryData, "", "    ")
	if err != nil {
		return err
	}
	if err := writeAtomic(cachePath, payload); err != nil {
		return err
	}
	logEvent("refresh", key, keyHash, "")
	return nil
}

func Delete(key string) {
	if !Enabled() {
		return
	}
	cachePath, keyHash, err := cachePathForKey(key)
	if err != nil {
		return
	}
	if err := os.Remove(cachePath); err == nil {
		logEvent("invalidate", key, keyHash, "")
	}
}

func WithLock(key string, fn func() error) error {
	if !Enabled() {
		return fn()
	}
	lockPath, keyHash, err := lockPathForKey(key)
	if err != nil {
		return err
	}
	start := time.Now()
	waitLogged := false
	for {
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = io.WriteString(lockFile, time.Now().UTC().Format(time.RFC3339Nano))
			_ = lockFile.Close()
			logEvent("lock_acquired", key, keyHash, "")
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		info, statErr := os.Stat(lockPath)
		if statErr == nil && time.Since(info.ModTime()) > cfg.LockTimeout {
			_ = os.Remove(lockPath)
			logEvent("lock_stale", key, keyHash, "")
			continue
		}
		if !waitLogged {
			logEvent("lock_wait", key, keyHash, "")
			waitLogged = true
		}
		if time.Since(start) > cfg.LockTimeout {
			logEvent("lock_timeout", key, keyHash, "")
			return fmt.Errorf("cache lock timeout")
		}
		time.Sleep(cfg.LockPollInterval)
	}
	defer func() {
		_ = os.Remove(lockPath)
	}()
	return fn()
}

func defaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "github-apps-trampoline")
	}
	return filepath.Join(os.TempDir(), "github-apps-trampoline-cache")
}

func cachePathForKey(key string) (string, string, error) {
	dir, keyHash, err := dirForKey(key)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.json", keyHash)), keyHash, nil
}

func lockPathForKey(key string) (string, string, error) {
	dir, keyHash, err := dirForKey(key)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.lock", keyHash)), keyHash, nil
}

func dirForKey(key string) (string, string, error) {
	keyType := "misc"
	if idx := strings.Index(key, ":"); idx > 0 {
		keyType = key[:idx]
	}
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:12])
	dir := filepath.Join(cfg.Dir, keyType)
	return dir, keyHash, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func logEvent(event, key, keyHash, details string) {
	message := fmt.Sprintf("cache %s key=%s", event, key)
	if details != "" {
		message = fmt.Sprintf("%s details=%s", message, details)
	}
	logger.Filef(message)
	stderrMessage := fmt.Sprintf("cache %s key=%s", event, keyHash)
	if details != "" {
		stderrMessage = fmt.Sprintf("%s details=%s", stderrMessage, details)
	}
	logger.Stderrf(stderrMessage)
}
