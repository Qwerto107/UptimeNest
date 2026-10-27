package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
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

func writePublicResponse(w http.ResponseWriter, r *http.Request, body []byte) {
	etag := responseETag(body)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}

func writePublicJSON(w http.ResponseWriter, r *http.Request, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		fail(w, err)
		return
	}
	writePublicResponse(w, r, append(body, '\n'))
}

func publicHeartbeatBucket(count int) int {
	for _, bucket := range []int{30, 60, 90, 180, 360, 720, 1000} {
		if count <= bucket {
			return bucket
		}
	}
	return 1000
}
