package crypto

import (
	"fmt"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	masterPassword := "test-master-password-12345"
	plaintext := "my-secret-api-key-abcdef123456"

	t.Run("Basic encrypt/decrypt cycle", func(t *testing.T) {
		encrypted, err := Encrypt(plaintext, masterPassword)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}

		if !IsEncrypted(encrypted) {
			t.Error("Encrypted value should be identified as encrypted")
		}

		if encrypted == plaintext {
			t.Error("Encrypted value should differ from plaintext")
		}

		decrypted, err := Decrypt(encrypted, masterPassword)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("Decrypted value mismatch: got %q, want %q", decrypted, plaintext)
		}
	})

	t.Run("Empty plaintext should return error", func(t *testing.T) {
		_, err := Encrypt("", masterPassword)
		if err == nil {
			t.Error("Expected error for empty plaintext")
		}
	})

	t.Run("Empty password should return error", func(t *testing.T) {
		_, err := Encrypt(plaintext, "")
		if err == nil {
			t.Error("Expected error for empty master password")
		}
	})

	t.Run("Invalid ciphertext format", func(t *testing.T) {
		_, err := Decrypt("not-encrypted-value", masterPassword)
		if err == nil {
			t.Error("Expected error for invalid ciphertext format")
		}
	})

	t.Run("Wrong password should fail", func(t *testing.T) {
		encrypted, _ := Encrypt(plaintext, masterPassword)
		
		_, err := Decrypt(encrypted, "wrong-password")
		if err == nil {
			t.Error("Expected decryption failure with wrong password")
		}
	})
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct{
		name     string
		value    string
		expected bool
	}{
		{"Valid encrypted prefix", "enc:v1:aes256:gcm:abc:def:ghi:jkl", true},
		{"Plain text", "hello world", false},
		{"Empty string", "", false},
		{"Short string", "enc:", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEncrypted(tt.value)
			if result != tt.expected {
				t.Errorf("IsEncrypted(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestValidateMasterPassword(t *testing.T) {
	tests := []struct{
		password string
		valid    bool
	}{
		{"Valid complex password: Abcdef1!@#", true},
		{"Too short: short", false},
		{"No uppercase: lowercase1!", false},
		{"No lowercase: UPPERCASE1!", false},
		{"No digit: NoDigits!!", false},
		{"No special char: Abcdefgh12", false},
		{"Valid with 12 chars: Abcd1234!@#", true},
	}

	for _, tt := range tests {
		t.Run(tt.password[:min(len(tt.password), 20)], func(t *testing.T) {
			result := ValidateMasterPassword(tt.password)
			if result != tt.valid {
				t.Errorf("ValidateMasterPassword(%q) = %v, want %v", tt.password, result, tt.valid)
			}
		})
	}
}

func TestGenerateMasterPassword(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprintf("Generation_%d", i), func(t *testing.T) {
			password, err := GenerateMasterPassword()
			if err != nil {
				t.Fatalf("GenerateMasterPassword failed: %v", err)
			}

			if len(password) < 32 {
				t.Errorf("Generated password too short: %d chars", len(password))
			}

			if !ValidateMasterPassword(password) {
				t.Errorf("Generated password did not pass validation: %s", password)
			}
		})
	}
}

func TestMultipleEncryptionsUnique(t *testing.T) {
	masterPassword := "test-password-unique"
	plaintext := "same-plaintext"

	encryptions := make(map[string]bool)

	for i := 0; i < 100; i++ {
		encrypted, err := Encrypt(plaintext, masterPassword)
		if err != nil {
			t.Fatalf("Encryption %d failed: %v", i, err)
		}

		if encryptions[encrypted] {
			t.Errorf("Duplicate encryption at iteration %d - IV should be unique", i)
		}
		encryptions[encrypted] = true
	}
}

func BenchmarkEncrypt(b *testing.B) {
	masterPassword := "benchmark-master-password"
	plaintext := "benchmark-api-key-for-performance-testing"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Encrypt(plaintext, masterPassword)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	benchMasterPassword := "benchmark-master-password"
	benchPlaintext := "benchmark-api-key-for-performance-testing"
	benchEncrypted, _ := Encrypt(benchPlaintext, benchMasterPassword)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decrypt(benchEncrypted, benchMasterPassword)
	}
}
