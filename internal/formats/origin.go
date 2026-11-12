package formats

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestBaseURL only selects a configured HTTPS origin. Forwarded headers and
// arbitrary Host values cannot become package download URLs.
func RequestBaseURL(c *gin.Context, canonical string, origins []string) string {
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err == nil && u.Scheme == "https" && u.User == nil && u.Host != "" &&
			(u.Path == "" || u.Path == "/") && u.RawQuery == "" && u.Fragment == "" && strings.EqualFold(c.Request.Host, u.Host) {
			return "https://" + u.Host
		}
	}
	return strings.TrimRight(canonical, "/")
}

func (d Deps) RequestBaseURL(c *gin.Context) string {
	return RequestBaseURL(c, d.BaseURL, d.PublicOrigins)
}

// RequestGroupIndexMerger lets a format generate links for the approved origin
// without mutating the handler shared by simultaneous requests.
type RequestGroupIndexMerger interface {
	MergeGroupIndexForRequest(*gin.Context, string, string, []GroupIndexPart) ([]byte, string, error)
}
