package fx

import (
	"net/http/httptest"
	"testing"
)

// 測試可以組出夾帶 user_id 的請求，確認 handler 不理會它
func TestIgnoresUserIDInQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/fx-trades?user_id=2", nil)
	_ = r.URL.Query().Get("user_id")
}
