package crypto

import (
	"context"
	"fmt"
	"strings"

	"openhijack/internal/errors"
)

type SecureConfigManager struct {
	store       SecretStore
	masterKey   string
	autoEncrypt bool
}

func NewSecureConfigManager(store SecretStore, masterKey string, autoEncrypt bool) *SecureConfigManager {
	return &SecureConfigManager{
		store:       store,
		masterKey:   masterKey,
		autoEncrypt: autoEncrypt,
	}
}

func (m *SecureConfigManager) EncryptField(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}
	return Encrypt(plaintext, m.masterKey)
}

func (m *SecureConfigManager) DecryptField(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}
	return Decrypt(ciphertext, m.masterKey)
}

func (m *SecureConfigManager) ProcessConfigForSave(config map[string]interface{}) (map[string]interface{}, []error) {
	result := make(map[string]interface{})
	var errs []error

	sensitiveFields := []string{
		"api_key",
		"auth_key",
		"password",
		"secret",
		"token",
		"private_key",
	}

	for key, value := range config {
		strValue, ok := value.(string)
		if !ok {
			result[key] = value
			continue
		}

		isSensitive := false
		for _, field := range sensitiveFields {
			if strings.HasSuffix(key, field) || strings.Contains(strings.ToLower(key), field) {
				isSensitive = true
				break
			}
		}

		if isSensitive && m.autoEncrypt && strValue != "" && !IsEncrypted(strValue) {
			encrypted, err := m.EncryptField(strValue)
			if err != nil {
				errs = append(errs, errors.ErrInternalf("encrypt field '%s' failed: %v", key, err))
				result[key] = strValue
			} else {
				result[key] = encrypted
			}
		} else {
			result[key] = value
		}
	}

	return result, errs
}

func (m *SecureConfigManager) ProcessConfigForLoad(config map[string]interface{}) (map[string]interface{}, []error) {
	result := make(map[string]interface{})
	var errs []error

	for key, value := range config {
		strValue, ok := value.(string)
		if !ok {
			result[key] = value
			continue
		}

		if IsEncrypted(strValue) {
			decrypted, err := m.DecryptField(strValue)
			if err != nil {
				errs = append(errs, errors.ErrInternalf("decrypt field '%s' failed: %v", key, err))
				result[key] = strValue
			} else {
				result[key] = decrypted
			}
		} else {
			result[key] = value
		}
	}

	return result, errs
}

func (m *SecureConfigManager) StoreSecret(ctx context.Context, key, value string) error {
	encrypted, err := m.EncryptField(value)
	if err != nil {
		return fmt.Errorf("encrypt secret for storage failed: %w", err)
	}
	return m.store.Set(ctx, key, encrypted)
}

func (m *SecureConfigManager) RetrieveSecret(ctx context.Context, key string) (string, error) {
	value, err := m.store.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("retrieve secret from storage failed: %w", err)
	}
	return m.DecryptField(value)
}

func (m *SecureConfigManager) DeleteSecret(ctx context.Context, key string) error {
	return m.store.Delete(ctx, key)
}

func (m *SecureConfigManager) ListSecrets(ctx context.Context) ([]string, error) {
	return m.store.List(ctx)
}

func (m *SecureConfigManager) NeedsMigration(config map[string]interface{}) bool {
	sensitiveKeywords := []string{"api_key", "auth_key", "password", "secret"}

	for key, value := range config {
		strValue, ok := value.(string)
		if !ok || strValue == "" || IsEncrypted(strValue) {
			continue
		}

		lowerKey := strings.ToLower(key)
		for _, keyword := range sensitiveKeywords {
			if strings.Contains(lowerKey, keyword) {
				return true
			}
		}
	}

	return false
}

func (m *SecureConfigManager) Close() error {
	if closer, ok := m.store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type MigrationResult struct {
	MigratedCount int
	SkippedCount  int
	Errors        []error
}

func (m *SecureConfigManager) MigrateConfigToEncrypted(config map[string]interface{}) (*MigrationResult, error) {
	result := &MigrationResult{}

	sensitiveFields := []string{"api_key", "auth_key", "password", "secret"}

	for key, value := range config {
		strValue, ok := value.(string)
		if !ok {
			result.SkippedCount++
			continue
		}

		if strValue == "" || IsEncrypted(strValue) {
			result.SkippedCount++
			continue
		}

		isSensitive := false
		lowerKey := strings.ToLower(key)
		for _, field := range sensitiveFields {
			if strings.Contains(lowerKey, field) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			encrypted, err := m.EncryptField(strValue)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("migrate field '%s' failed: %w", key, err))
			} else {
				config[key] = encrypted
				result.MigratedCount++
			}
		} else {
			result.SkippedCount++
		}
	}

	return result, nil
}
