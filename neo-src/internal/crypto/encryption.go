// Package crypto wraps symmetric encryption helpers for at-rest secrets.
//
// Port of `src/core/utils/encryption.zig` (little_timer).  Maps the Zig
// `std.crypto.aead.aes_gcm.Aes256Gcm` to Go's stdlib `crypto/aes` +
// `crypto/cipher` GCM, and `std.crypto.pwhash.pbkdf2` to
// `golang.org/x/crypto/pbkdf2`.
//
// Wire format (encrypted blob produced by EncryptWithPassword):
//
//	salt (16) || nonce (12) || ciphertext (N) || gcm_tag (16)
//
// This matches the Zig source's byte layout; tests asserting the layout
// stay portable across builds.
//
// API:
//
//   - Encrypt(plaintext, key, nonce) / Decrypt(blob, key) — raw AES-GCM
//     round-trip.  Output of Encrypt is nonce(12) || ciphertext || tag(16).
//   - EncryptWithPassword(plaintext, password) / DecryptWithPassword — adds
//     PBKDF2-HMAC-SHA256 key derivation with a 16-byte salt.
//   - DeriveKey(password, salt) → 32-byte key (exposed for tests).
//   - GenerateKey / GenerateNonce / GenerateSalt — random byte generation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// -----------------------------------------------------------------------------
// Constants — mirror Zig AES256GCM_KEY_SIZE / NONCE_SIZE / TAG_SIZE.
// -----------------------------------------------------------------------------

const (
	AES256GCMKeySize   = 32
	AES256GCMNonceSize = 12
	AES256GCMTagSize   = 16
	SaltSize           = 16
	PBKDF2Iterations   = 100_000
)

// CryptoError 对应 Zig 源码中的 `pub const CryptoError = error{...}`，
// 是一个类型化哨兵错误，可通过 errors.Is / errors.As 进行匹配。
type CryptoError string

const (
	ErrInvalidKeyLength     CryptoError = "invalid key length"
	ErrInvalidNonceLength   CryptoError = "invalid nonce length"
	ErrAuthenticationFailed CryptoError = "authentication failed"
	ErrOutOfMemory          CryptoError = "out of memory"
)

func (e CryptoError) Error() string { return string(e) }

// -----------------------------------------------------------------------------
// Random helpers.
// -----------------------------------------------------------------------------

// GenerateKey 返回一个新生成的 32 字节 AES-256 密钥。对应 Zig 中的
// `pub fn generateKey()`。
func GenerateKey() []byte { return randomBytes(AES256GCMKeySize) }

// GenerateNonce 返回一个新生成的 12 字节 GCM nonce。
func GenerateNonce() []byte { return randomBytes(AES256GCMNonceSize) }

// GenerateSalt 返回一个新生成的 16 字节 PBKDF2 salt。
func GenerateSalt() []byte { return randomBytes(SaltSize) }

// randomBytes reads n bytes from crypto/rand; on error (extremely rare —
// only if the OS RNG fails) it panics, matching Zig's
// `std.crypto.random.bytes(&key)` behaviour of treating RNG failure as fatal.
func randomBytes(n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic(fmt.Errorf("crypto: read random bytes: %w", err))
	}
	return buf
}

// DeriveKey 使用 PBKDF2-HMAC-SHA256(password, salt, 100_000) 派生 32 字节密钥。
// 对应 Zig 中的 `pub fn deriveKey`：Zig 端使用 HmacSha256，本实现改用
// `crypto/sha256` + `golang.org/x/crypto/pbkdf2`。
func DeriveKey(password, salt []byte) ([]byte, error) {
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("%w: salt must be %d bytes, got %d",
			ErrInvalidKeyLength, SaltSize, len(salt))
	}
	return pbkdf2.Key(password, salt, PBKDF2Iterations, AES256GCMKeySize, sha256.New), nil
}

// -----------------------------------------------------------------------------
// Raw AES-256-GCM (caller supplies the 32-byte key and the 12-byte nonce).
// Wire format matches Zig: nonce || ciphertext || tag.  Output of Encrypt
// is len(plaintext)+NonceSize+TagSize bytes.
// -----------------------------------------------------------------------------

// Encrypt 使用 AES-256-GCM 与给定的 key/nonce 加密明文。返回的二进制数据
// 布局为 nonce || ciphertext || tag。
func Encrypt(plaintext, key, nonce []byte) ([]byte, error) {
	if len(key) != AES256GCMKeySize {
		return nil, fmt.Errorf("%w: key must be %d bytes, got %d",
			ErrInvalidKeyLength, AES256GCMKeySize, len(key))
	}
	if len(nonce) != AES256GCMNonceSize {
		return nil, fmt.Errorf("%w: nonce must be %d bytes, got %d",
			ErrInvalidNonceLength, AES256GCMNonceSize, len(nonce))
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	// Seal appends ciphertext + tag to its first argument; we pre-size the
	// destination as nonce+ct+tag so the result is one allocation.
	out := make([]byte, 0, len(nonce)+len(plaintext)+AES256GCMTagSize)
	out = append(out, nonce...)
	sealed := gcm.Seal(out, nonce, plaintext, nil)
	return sealed, nil
}

// Decrypt 解密由 Encrypt 产生的二进制数据。当 GCM tag 校验失败时返回
// ErrAuthenticationFailed。
func Decrypt(blob, key []byte) ([]byte, error) {
	if len(key) != AES256GCMKeySize {
		return nil, fmt.Errorf("%w: key must be %d bytes, got %d",
			ErrInvalidKeyLength, AES256GCMKeySize, len(key))
	}
	if len(blob) < AES256GCMNonceSize+AES256GCMTagSize {
		return nil, fmt.Errorf("%w: blob too short (%d bytes)",
			ErrAuthenticationFailed, len(blob))
	}
	nonce := blob[:AES256GCMNonceSize]
	body := blob[AES256GCMNonceSize:]
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}
	return pt, nil
}

// -----------------------------------------------------------------------------
// Password-based helpers — DeriveKey + Encrypt/Decrypt bundled together.
// Wire format: salt(16) || nonce(12) || ciphertext(N) || tag(16).
// -----------------------------------------------------------------------------

// EncryptWithPassword 从 password+salt 派生 32 字节密钥，再加密明文。
// salt 在内部生成，调用方需持久化返回的 blob 以便后续恢复明文。
func EncryptWithPassword(plaintext, password []byte) ([]byte, error) {
	salt := GenerateSalt()
	key, err := DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	nonce := GenerateNonce()
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	// Wire layout: salt || nonce || sealed (sealed = ct || tag)
	out := make([]byte, 0, SaltSize+AES256GCMNonceSize+len(sealed))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// DecryptWithPassword 解密由 EncryptWithPassword 产生的二进制数据。
func DecryptWithPassword(blob, password []byte) ([]byte, error) {
	if len(blob) < SaltSize+AES256GCMNonceSize+AES256GCMTagSize {
		return nil, fmt.Errorf("%w: blob too short (%d bytes)",
			ErrAuthenticationFailed, len(blob))
	}
	salt := blob[:SaltSize]
	nonce := blob[SaltSize : SaltSize+AES256GCMNonceSize]
	body := blob[SaltSize+AES256GCMNonceSize:]
	key, err := DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	pt, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}
	return pt, nil
}

// -----------------------------------------------------------------------------
// Internal helpers.
// -----------------------------------------------------------------------------

// newGCM constructs an AES-256-GCM cipher from the supplied 32-byte key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes.NewCipher: %w", err)
	}
	return cipher.NewGCM(block)
}