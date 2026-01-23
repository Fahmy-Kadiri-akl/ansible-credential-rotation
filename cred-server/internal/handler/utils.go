package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/rs/zerolog/log"
)

// generateID generates a random ID
func generateID() string {
	b := make([]byte, constants.CREDENTIAL__ID__LENGTH__BYTES__DEFAULT)
	if _, err := rand.Read(b); err != nil {
		log.Error().Err(err).Msg("Failed to generate random ID")
	}
	return hex.EncodeToString(b)
}

// generateSecureToken generates a secure random token
func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		log.Error().Err(err).Msg("Failed to generate secure token")
	}
	return base64.URLEncoding.EncodeToString(b)[:length]
}

// generateSecurePassword generates a secure random password
func generateSecurePassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		log.Error().Err(err).Msg("Failed to generate secure password")
	}
	for i := range b {
		b[i] = constants.CREDENTIAL__PASSWORD__CHARSET[int(b[i])%len(constants.CREDENTIAL__PASSWORD__CHARSET)]
	}
	return string(b)
}
