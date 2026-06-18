package zpay

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"strings"
)

func Sign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}

	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}

func Verify(params map[string]string, key string) bool {
	got := params["sign"]
	if got == "" {
		return false
	}
	want := Sign(params, key)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
