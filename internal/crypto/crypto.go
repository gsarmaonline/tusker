package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// Encryptor performs envelope encryption using AES-256-GCM.
// A root key encrypts per-tenant data keys; data keys encrypt secrets and tokens.
type Encryptor struct {
	rootKey []byte
}

// NewEncryptor creates an Encryptor from a 32-byte hex-encoded root key.
func NewEncryptor(rootKeyHex string) (*Encryptor, error) {
	key, err := hex.DecodeString(rootKeyHex)
	if err != nil {
		return nil, errors.New("ROOT_ENCRYPTION_KEY must be hex-encoded")
	}
	if len(key) != 32 {
		return nil, errors.New("ROOT_ENCRYPTION_KEY must be 32 bytes (64 hex chars)")
	}
	return &Encryptor{rootKey: key}, nil
}

// GenerateDataKey generates a random 32-byte data key and returns it
// in both plaintext (for immediate use) and encrypted (for storage) form.
func (e *Encryptor) GenerateDataKey() (plaintext []byte, encrypted []byte, err error) {
	key := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, err
	}
	encrypted, err = encrypt(e.rootKey, key)
	if err != nil {
		return nil, nil, err
	}
	return key, encrypted, nil
}

// DecryptDataKey decrypts a stored data key using the root key.
func (e *Encryptor) DecryptDataKey(encrypted []byte) ([]byte, error) {
	return decrypt(e.rootKey, encrypted)
}

// EncryptWithDataKey encrypts plaintext using a tenant's plaintext data key.
func EncryptWithDataKey(dataKey, plaintext []byte) ([]byte, error) {
	return encrypt(dataKey, plaintext)
}

// DecryptWithDataKey decrypts ciphertext using a tenant's plaintext data key.
func DecryptWithDataKey(dataKey, ciphertext []byte) ([]byte, error) {
	return decrypt(dataKey, ciphertext)
}

// encrypt performs AES-256-GCM encryption. Output format: [nonce(12) | ciphertext+tag].
func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt performs AES-256-GCM decryption. Expects [nonce(12) | ciphertext+tag].
func decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
