package crypto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"openhijack/internal/errors"
)

type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context) ([]string, error)
	Close() error
}

var (
	globalStore     SecretStore
	globalStoreOnce sync.Once
	storeInitError  error
)

func GetGlobalStore() (SecretStore, error) {
	globalStoreOnce.Do(func() {
		globalStore, storeInitError = NewSecretStore()
	})
	return globalStore, storeInitError
}

func InitGlobalStore(store SecretStore) {
	globalStore = store
	globalStoreOnce = sync.Once{}
}

type FileStore struct {
	basePath   string
	masterKey  []byte
	mu         sync.RWMutex
	cachedData map[string]string
}

func NewFileStore(basePath string, masterPassword string) (*FileStore, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("create base path failed: %w", err)
	}

	key := []byte(masterPassword)
	return &FileStore{
		basePath:   basePath,
		masterKey:  key,
		cachedData: make(map[string]string),
	}, nil
}

func (s *FileStore) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if val, ok := s.cachedData[key]; ok {
		return val, nil
	}

	filePath := s.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.ErrNotFoundf("secret '%s' not found", key)
		}
		return "", fmt.Errorf("read secret file failed: %w", err)
	}

	encrypted := string(data)
	if !IsEncrypted(encrypted) {
		s.cachedData[key] = encrypted
		return encrypted, nil
	}

	plaintext, err := Decrypt(encrypted, string(s.masterKey))
	if err != nil {
		return "", fmt.Errorf("decrypt secret failed: %w", err)
	}

	s.cachedData[key] = plaintext
	return plaintext, nil
}

func (s *FileStore) Set(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encrypted, err := Encrypt(value, string(s.masterKey))
	if err != nil {
		return fmt.Errorf("encrypt secret failed: %w", err)
	}

	filePath := s.getFilePath(key)
	if err := os.WriteFile(filePath, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("write secret file failed: %w", err)
	}

	s.cachedData[key] = value
	return nil
}

func (s *FileStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getFilePath(key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete secret file failed: %w", err)
	}

	delete(s.cachedData, key)
	return nil
}

func (s *FileStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	_, exists := s.cachedData[key]
	s.mu.RUnlock()

	if exists {
		return true, nil
	}

	filePath := s.getFilePath(key)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *FileStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("read secrets directory failed: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".enc" {
			keyName := entry.Name()[:len(entry.Name())-4]
			keys = append(keys, keyName)
		}
	}

	return keys, nil
}

func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k := range s.cachedData {
		delete(s.cachedData, k)
	}
	zeroKey(s.masterKey)
	return nil
}

func (s *FileStore) getFilePath(key string) string {
	safeKey := sanitizeKey(key)
	return filepath.Join(s.basePath, safeKey+".enc")
}

type EnvVarStore struct {
	prefix string
}

func NewEnvVarStore(prefix string) *EnvVarStore {
	return &EnvVarStore{prefix: prefix}
}

func (s *EnvVarStore) Get(_ context.Context, key string) (string, error) {
	envKey := s.prefix + key
	value := os.Getenv(envKey)
	if value == "" {
		return "", errors.ErrNotFoundf("environment variable '%s' not set", envKey)
	}
	return value, nil
}

func (s *EnvVarStore) Set(_ context.Context, key, value string) error {
	envKey := s.prefix + key
	return os.Setenv(envKey, value)
}

func (s *EnvVarStore) Delete(_ context.Context, key string) error {
	envKey := s.prefix + key
	return os.Unsetenv(envKey)
}

func (s *EnvVarStore) Exists(_ context.Context, key string) (bool, error) {
	envKey := s.prefix + key
	_, exists := os.LookupEnv(envKey)
	return exists, nil
}

func (s *EnvVarStore) List(_ context.Context) ([]string, error) {
	var keys []string
	for _, env := range os.Environ() {
		if len(env) > len(s.prefix) && env[:len(s.prefix)] == s.prefix {
			key := env[len(s.prefix):]
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (s *EnvVarStore) Close() error { return nil }

func NewSecretStore() (SecretStore, error) {
	if os.Getenv("OPENHIJACK_KEYSTORE") == "env" {
		return NewEnvVarStore("OPENHIJACK_SECRET_"), nil
	}

	if isDesktopEnvironment() {
		store, err := tryKeyringStore()
		if err == nil {
			return store, nil
		}
	}

	defaultPath := getDefaultSecretPath()
	masterPassword := getMasterPasswordFromEnv()
	if masterPassword == "" {
		masterPassword = "openhijack-default-master-key-please-change-me"
	}

	return NewFileStore(defaultPath, masterPassword)
}

func isDesktopEnvironment() bool {
	display := os.Getenv("DISPLAY")
	wayland := os.Getenv("WAYLAND_DISPLAY")
	sessionType := os.Getenv("XDG_SESSION_TYPE")

	return display != "" || wayland != "" || sessionType == "wayland" || sessionType == "x11"
}

func tryKeyringStore() (SecretStore, error) {
	return nil, errors.New(errors.ErrNotImplemented, "keyring not available")
}

func getDefaultSecretPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".openhijack-secrets"
	}
	return filepath.Join(homeDir, ".local", "share", "openhijack", "secrets")
}

func getMasterPasswordFromEnv() string {
	return os.Getenv("OPENHIJACK_MASTER_PASSWORD")
}

func sanitizeKey(key string) string {
	result := make([]byte, 0, len(key))
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			result = append(result, byte(c))
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
