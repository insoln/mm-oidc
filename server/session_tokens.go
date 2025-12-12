package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type sessionTokenRecord struct {
	SessionID    string `json:"session_id"`
	SessionToken string `json:"session_token"`
	UserID       string `json:"user_id"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int64  `json:"created_at"`
}

func (p *Plugin) persistSessionTokens(session *model.Session, tokens *tokenResponse, cfg *Configuration) error {
	if session == nil || tokens == nil {
		return fmt.Errorf("missing session or token response")
	}
	if cfg == nil {
		return fmt.Errorf("configuration unavailable")
	}
	if strings.TrimSpace(session.Token) == "" {
		return fmt.Errorf("session token missing")
	}

	record := &sessionTokenRecord{
		SessionID:    session.Id,
		SessionToken: session.Token,
		UserID:       session.UserId,
		IDToken:      tokens.IDToken,
		RefreshToken: tokens.RefreshToken,
		CreatedAt:    time.Now().Unix(),
	}

	return p.saveSessionTokenRecord(record, cfg)
}

func (p *Plugin) saveSessionTokenRecord(record *sessionTokenRecord, cfg *Configuration) error {
	if record == nil {
		return fmt.Errorf("session token record is nil")
	}
	if cfg == nil || strings.TrimSpace(cfg.ClientSecret) == "" {
		return fmt.Errorf("client secret is required for encryption")
	}
	if strings.TrimSpace(record.SessionToken) == "" {
		return fmt.Errorf("session token missing")
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session tokens: %w", err)
	}

	ciphertext, err := encryptForStorage(payload, cfg.ClientSecret)
	if err != nil {
		return fmt.Errorf("encrypt session tokens: %w", err)
	}

	if appErr := p.API.KVSet(sessionTokenKey(record.SessionToken), ciphertext); appErr != nil {
		return fmt.Errorf("persist session tokens: %w", appErr)
	}

	return nil
}

func (p *Plugin) loadSessionTokenRecord(sessionToken string, cfg *Configuration) (*sessionTokenRecord, error) {
	if strings.TrimSpace(sessionToken) == "" {
		return nil, nil
	}
	ciphertext, appErr := p.API.KVGet(sessionTokenKey(sessionToken))
	if appErr != nil {
		return nil, fmt.Errorf("load session tokens: %w", appErr)
	}
	if len(ciphertext) == 0 {
		return nil, nil
	}
	if cfg == nil || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("client secret is required for decryption")
	}

	plaintext, err := decryptFromStorage(ciphertext, cfg.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt session tokens: %w", err)
	}

	var record sessionTokenRecord
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return nil, fmt.Errorf("decode session tokens: %w", err)
	}

	return &record, nil
}

func (p *Plugin) deleteSessionTokenRecord(sessionToken string) error {
	if strings.TrimSpace(sessionToken) == "" {
		return nil
	}
	if err := p.API.KVDelete(sessionTokenKey(sessionToken)); err != nil {
		return fmt.Errorf("delete session tokens: %w", err)
	}
	return nil
}

func sessionTokenKey(token string) string {
	return "sessiontokens:" + token
}

func encryptForStorage(plaintext []byte, secret string) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext is empty")
	}
	key, err := deriveEncryptionKey(secret)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptFromStorage(ciphertext []byte, secret string) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext is empty")
	}
	key, err := deriveEncryptionKey(secret)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	payload := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}

	return plaintext, nil
}

func deriveEncryptionKey(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, fmt.Errorf("encryption secret is empty")
	}
	digest := sha256.Sum256([]byte(trimmed))
	return digest[:], nil
}
