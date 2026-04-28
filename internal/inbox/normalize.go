package inbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func HashPayload(raw, normalized map[string]any) (string, error) {
	data, err := json.Marshal(map[string]any{"raw": raw, "normalized": normalized})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
