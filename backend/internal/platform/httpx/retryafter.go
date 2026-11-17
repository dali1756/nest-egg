package httpx

import (
	"net/http"
	"strconv"
	"time"
)

// 讓使用者知道還要等幾秒，無條件進位，最少 1 秒，提早重試只會再被擋一次
func SetRetryAfter(w http.ResponseWriter, wait time.Duration) {
	seconds := max(int((wait+time.Second-1)/time.Second), 1)
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
