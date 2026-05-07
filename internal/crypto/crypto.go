package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	Version         = "v1"
	Algorithm       = "aes256"
	Mode            = "gcm"
	KeySize         = 32
	NonceSize       = 12
	TagSize         = 16
	SaltSize        = 16
	PBKDF2Iterations = 600_000
	Prefix          = "enc:"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	ErrDecryptFailed     = errors.New("decryption failed: wrong password or corrupted data")
	ErrEmptyPlaintext   = errors.New("plaintext cannot be empty")
)

type EncryptedValue struct {
	Version   string `json:"version"`
	Algorithm string `json:"algorithm"`
	Mode      string `json:"mode"`
	Salt      string `json:"salt"`
	IV        string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
	Tag       string `json:"tag"`
}

func Encrypt(plaintext string, masterPassword string) (string, error) {
	if plaintext == "" {
		return "", ErrEmptyPlaintext
	}
	if masterPassword == "" {
		return "", errors.New("master password cannot be empty")
	}

	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt failed: %w", err)
	}

	key, aesGCM, err := deriveKeyAndGCM(masterPassword, salt)
	if err != nil {
		return "", err
	}
	defer zeroKey(key)

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce failed: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)

	ctLen := len(ciphertext) - TagSize
	ct := ciphertext[:ctLen]
	tag := ciphertext[ctLen:]

	result := fmt.Sprintf("%s%s:%s:%s:%s:%s:%s:%s",
		Prefix,
		Version,
		Algorithm,
		Mode,
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ct),
		base64.StdEncoding.EncodeToString(tag),
	)

	return result, nil
}

func Decrypt(encryptedStr string, masterPassword string) (string, error) {
	if encryptedStr == "" {
		return "", ErrInvalidCiphertext
	}
	if masterPassword == "" {
		return "", errors.New("master password cannot be empty")
	}

	enc, err := parseEncryptedString(encryptedStr)
	if err != nil {
		return "", err
	}

	salt, err := base64.StdEncoding.DecodeString(enc.Salt)
	if err != nil {
		return "", fmt.Errorf("decode salt failed: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(enc.IV)
	if err != nil {
		return "", fmt.Errorf("decode iv failed: %w", err)
	}

	ct, err := base64.StdEncoding.DecodeString(enc.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext failed: %w", err)
	}

	tag, err := base64.StdEncoding.DecodeString(enc.Tag)
	if err != nil {
		return "", fmt.Errorf("decode tag failed: %w", err)
	}

	key, aesGCM, err := deriveKeyAndGCM(masterPassword, salt)
	if err != nil {
		return "", err
	}
	defer zeroKey(key)

	ciphertextWithTag := append(ct, tag...)
	plaintext, err := aesGCM.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}

	return string(plaintext), nil
}

func deriveKeyAndGCM(password string, salt []byte) (key []byte, gcm cipher.AEAD, err error) {
	key = pbkdf2.Key(
		[]byte(password),
		salt,
		PBKDF2Iterations,
		KeySize,
		sha256.New,
	)

	block, cipherErr := aes.NewCipher(key)
	if cipherErr != nil {
		zeroKey(key)
		return nil, nil, fmt.Errorf("create cipher failed: %w", cipherErr)
	}

	gcm, gcmErr := cipher.NewGCM(block)
	if gcmErr != nil {
		zeroKey(key)
		return nil, nil, fmt.Errorf("create gcm failed: %w", gcmErr)
	}

	return key, gcm, nil
}

func IsEncrypted(value string) bool {
	return len(value) > 4 && value[:4] == Prefix
}

func parseEncryptedString(s string) (*EncryptedValue, error) {
	if !IsEncrypted(s) {
		return nil, ErrInvalidCiphertext
	}

	parts := s[4:]
	var enc EncryptedValue

	n, err := fmt.Sscanf(parts, "%s:%s:%s:%s:%s:%s:%s",
		&enc.Version,
		&enc.Algorithm,
		&enc.Mode,
		&enc.Salt,
		&enc.IV,
		&enc.Ciphertext,
		&enc.Tag,
	)
	if err != nil || n != 7 {
		return nil, fmt.Errorf("%w: invalid format (expected 7 parts, got %d)", ErrInvalidCiphertext, n)
	}

	if enc.Version != Version {
		return nil, fmt.Errorf("unsupported version: %s (expected %s)", enc.Version, Version)
	}
	if enc.Algorithm != Algorithm {
		return nil, fmt.Errorf("unsupported algorithm: %s (expected %s)", enc.Algorithm, Algorithm)
	}
	if enc.Mode != Mode {
		return nil, fmt.Errorf("unsupported mode: %s (expected %s)", enc.Mode, Mode)
	}

	return &enc, nil
}

func GenerateMasterPassword() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate master password failed: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func ValidateMasterPassword(password string) bool {
	if len(password) < 16 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if !(hasUpper && hasLower && hasDigit && hasSpecial) {
		return false
	}

	charSetSize := 0
	if hasUpper {
		charSetSize += 26
	}
	if hasLower {
		charSetSize += 26
	}
	if hasDigit {
		charSetSize += 10
	}
	if hasSpecial {
		charSetSize += 33
	}

	entropy := float64(len(password)) * math.Log2(float64(charSetSize))
	if entropy < 60 {
		return false
	}

	if hasSequentialPattern(password) || hasRepeatingChars(password) {
		return false
	}

	return true
}

func hasSequentialPattern(s string) bool {
	seqPatterns := []string{
		"0123456789",
		"9876543210",
		"abcdefghijklmnopqrstuvwxyz",
		"zyxwvutsrqponmlkjihgfedcba",
		"qwertyuiop",
		"asdfghjkl",
		"zxcvbnm",
	}

	lower := strings.ToLower(s)
	for _, pattern := range seqPatterns {
		for i := 0; i <= len(pattern)-4; i++ {
			substr := pattern[i : i+4]
			if strings.Contains(lower, substr) {
				return true
			}
		}
	}
	return false
}

func hasRepeatingChars(s string) bool {
	if len(s) < 4 {
		return false
	}

	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			count++
			if count >= 4 {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

func zeroKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}

type Reader struct {
	source io.Reader
	buffer []byte
	pos    int
	err    error
}

func NewReader(source io.Reader, key string) (*Reader, error) {
	ciphertext, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("read source failed: %w", err)
	}

	plaintext, err := Decrypt(string(ciphertext), key)
	if err != nil {
		return nil, err
	}

	return &Reader{
		source: source,
		buffer: []byte(plaintext),
		pos:    0,
	}, nil
}

func (r *Reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.pos >= len(r.buffer) {
		return 0, io.EOF
	}
	n := copy(p, r.buffer[r.pos:])
	r.pos += n
	return n, nil
}

type Writer struct {
	dest     io.Writer
	key      string
	buffer   []byte
}

func NewWriter(dest io.Writer, key string) *Writer {
	return &Writer{
		dest:   dest,
		key:    key,
		buffer: make([]byte, 0),
	}
}

func (w *Writer) Write(p []byte) (int, error) {
	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *Writer) Close() error {
	encrypted, err := Encrypt(string(w.buffer), w.key)
	if err != nil {
		return fmt.Errorf("encrypt failed: %w", err)
	}
	_, err = w.dest.Write([]byte(encrypted))
	if err != nil {
		return fmt.Errorf("write encrypted data failed: %w", err)
	}
	return nil
}
