package formats

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestRequestBaseURL(t *testing.T) {
	for _, host := range []string{"registry.internal.example", "registry.example.com", "attacker.example", "registry.example.com.evil"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "https://"+host+"/", nil)
		c.Request.Header.Set("X-Forwarded-Host", "attacker.example")
		c.Request.Header.Set("X-Forwarded-Proto", "http")
		want := "https://registry.internal.example"
		if host == "registry.example.com" {
			want = "https://registry.example.com"
		}
		require.Equal(t, want, RequestBaseURL(c, "https://registry.internal.example", []string{"https://registry.internal.example", "https://registry.example.com"}))
	}
}
