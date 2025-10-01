package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// HashCalculator provides file hash calculation functionality
type HashCalculator struct{}

// NewHashCalculator creates a new hash calculator
func NewHashCalculator() *HashCalculator {
	return &HashCalculator{}
}

// CalculateHash calculates the hash of a file using the specified algorithm
func (h *HashCalculator) CalculateHash(filePath string, algorithm string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var hasher hash.Hash
	switch strings.ToLower(algorithm) {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	// Copy file content to hasher in chunks to handle large files efficiently
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// VerifyHash verifies a file against an expected hash
func (h *HashCalculator) VerifyHash(filePath string, expectedHash string, algorithm string) error {
	actualHash, err := h.CalculateHash(filePath, algorithm)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	if strings.ToLower(actualHash) != strings.ToLower(expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// GetSupportedAlgorithms returns a list of supported hash algorithms
func (h *HashCalculator) GetSupportedAlgorithms() []string {
	return []string{"md5", "sha1", "sha256", "sha512"}
}

// DetectHashAlgorithm attempts to detect the hash algorithm based on hash length
func (h *HashCalculator) DetectHashAlgorithm(hashValue string) string {
	hashValue = strings.TrimSpace(hashValue)
	switch len(hashValue) {
	case 32:
		return "md5"
	case 40:
		return "sha1"
	case 64:
		return "sha256"
	case 128:
		return "sha512"
	default:
		return "unknown"
	}
}
