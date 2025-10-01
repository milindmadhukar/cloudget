package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHashCalculator(t *testing.T) {
	calc := NewHashCalculator()
	if calc == nil {
		t.Fatal("NewHashCalculator() returned nil")
	}
}

func TestCalculateHash(t *testing.T) {
	calc := NewHashCalculator()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		algorithm string
		expected  string
	}{
		{"MD5", "md5", "65a8e27d8879283831b664bd8b7f0ad4"},
		{"SHA1", "sha1", "0a0a9f2a6772942557ab5355d76af442f8f65e01"},
		{"SHA256", "sha256", "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"},
		{"SHA512", "sha512", "374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.CalculateHash(testFile, tt.algorithm)
			if err != nil {
				t.Fatalf("CalculateHash(%s) failed: %v", tt.algorithm, err)
			}
			if result != tt.expected {
				t.Errorf("CalculateHash(%s) = %s, want %s", tt.algorithm, result, tt.expected)
			}
		})
	}
}

func TestCalculateHashUnsupportedAlgorithm(t *testing.T) {
	calc := NewHashCalculator()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = calc.CalculateHash(testFile, "unsupported")
	if err == nil {
		t.Error("Expected error for unsupported algorithm, got nil")
	}
	if err.Error() != "unsupported hash algorithm: unsupported" {
		t.Errorf("Expected 'unsupported hash algorithm' error, got: %v", err)
	}
}

func TestCalculateHashNonExistentFile(t *testing.T) {
	calc := NewHashCalculator()

	_, err := calc.CalculateHash("/nonexistent/file.txt", "md5")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestVerifyHash(t *testing.T) {
	calc := NewHashCalculator()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with correct hash
	err = calc.VerifyHash(testFile, "65a8e27d8879283831b664bd8b7f0ad4", "md5")
	if err != nil {
		t.Errorf("VerifyHash with correct hash failed: %v", err)
	}

	// Test with incorrect hash
	err = calc.VerifyHash(testFile, "incorrecthash", "md5")
	if err == nil {
		t.Error("Expected error for incorrect hash, got nil")
	}

	// Test case insensitive comparison
	err = calc.VerifyHash(testFile, "65A8E27D8879283831B664BD8B7F0AD4", "md5")
	if err != nil {
		t.Errorf("VerifyHash with uppercase hash failed: %v", err)
	}
}

func TestGetSupportedAlgorithms(t *testing.T) {
	calc := NewHashCalculator()
	algorithms := calc.GetSupportedAlgorithms()

	expected := []string{"md5", "sha1", "sha256", "sha512"}
	if len(algorithms) != len(expected) {
		t.Errorf("GetSupportedAlgorithms() returned %d algorithms, want %d", len(algorithms), len(expected))
	}

	for i, alg := range expected {
		if algorithms[i] != alg {
			t.Errorf("GetSupportedAlgorithms()[%d] = %s, want %s", i, algorithms[i], alg)
		}
	}
}

func TestDetectHashAlgorithm(t *testing.T) {
	calc := NewHashCalculator()

	tests := []struct {
		name     string
		hash     string
		expected string
	}{
		{"MD5", "65a8e27d8879283831b664bd8b7f0ad4", "md5"},
		{"SHA1", "0a0a9f2a6772942557ab5355d76af442f8f65e01", "sha1"},
		{"SHA256", "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f", "sha256"},
		{"SHA512", "374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387", "sha512"},
		{"Unknown", "tooshort", "unknown"},
		{"WithSpaces", "  65a8e27d8879283831b664bd8b7f0ad4  ", "md5"},
		{"Empty", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.DetectHashAlgorithm(tt.hash)
			if result != tt.expected {
				t.Errorf("DetectHashAlgorithm(%s) = %s, want %s", tt.hash, result, tt.expected)
			}
		})
	}
}

func TestCalculateHashLargeFile(t *testing.T) {
	calc := NewHashCalculator()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file larger than the buffer size (32KB)
	content := make([]byte, 64*1024) // 64KB
	for i := range content {
		content[i] = byte(i % 256)
	}

	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Calculate hash and verify it doesn't error
	hash, err := calc.CalculateHash(testFile, "md5")
	if err != nil {
		t.Fatalf("CalculateHash for large file failed: %v", err)
	}

	if len(hash) != 32 {
		t.Errorf("MD5 hash length = %d, want 32", len(hash))
	}
}
