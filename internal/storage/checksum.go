// Package storage provides storage management with multipart upload support.
package storage

import (
	"crypto/md5" //nolint:gosec // MD5 is used for data integrity checks, not cryptographic security
	"encoding/hex"
	"fmt"
	"io"
)

// ChecksumVerifier provides MD5-based integrity verification for uploaded parts.
type ChecksumVerifier struct{}

// NewChecksumVerifier creates a new ChecksumVerifier.
func NewChecksumVerifier() *ChecksumVerifier {
	return &ChecksumVerifier{}
}

// CalculateMD5 reads all data from r and returns the hex-encoded MD5 digest.
func (v *ChecksumVerifier) CalculateMD5(data io.Reader) (string, error) {
	h := md5.New() //nolint:gosec // MD5 used for data integrity, not security
	if _, err := io.Copy(h, data); err != nil {
		return "", fmt.Errorf("failed to calculate MD5: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyMD5 reads all data from r and returns true if its MD5 digest matches expectedMD5.
func (v *ChecksumVerifier) VerifyMD5(data io.Reader, expectedMD5 string) (bool, error) {
	actual, err := v.CalculateMD5(data)
	if err != nil {
		return false, err
	}
	return actual == expectedMD5, nil
}

// VerifyPartIntegrity verifies that the MD5 digest of data matches expectedMD5.
// It returns an error if the checksums do not match.
func (v *ChecksumVerifier) VerifyPartIntegrity(data []byte, expectedMD5 string) error {
	h := md5.New() //nolint:gosec // MD5 used for data integrity, not security
	if _, err := h.Write(data); err != nil {
		return fmt.Errorf("failed to hash data: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedMD5 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedMD5, actual)
	}
	return nil
}
