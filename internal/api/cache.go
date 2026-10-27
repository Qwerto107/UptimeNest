package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"regexp"
)

var hashedAsset = regexp.MustCompile(`^assets/.+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

func responseETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func contentETag(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return `"` + hex.EncodeToString(hash.Sum(nil)) + `"`, nil
}

func publicHeartbeatBucket(count int) int {
	for _, bucket := range []int{30, 60, 90, 180, 360, 720, 1000} {
		if count <= bucket {
			return bucket
		}
	}
	return 1000
}
