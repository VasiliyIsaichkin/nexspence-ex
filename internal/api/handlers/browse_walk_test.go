package handlers_test

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Existing artifact assertions now traverse the real paged branch API.
func doBrowse(t *testing.T, r *gin.Engine, base string) *httptest.ResponseRecorder {
	t.Helper()
	var walk func(string, int) *httptest.ResponseRecorder
	walk = func(path string, depth int) *httptest.ResponseRecorder {
		require.Less(t, depth, 30)
		result := browseTreeResp{}
		cursor := ""
		for {
			w := do(t, r, http.MethodGet, base+"?path="+url.QueryEscape(path)+"&cursor="+url.QueryEscape(cursor), nil)
			if w.Code != 200 {
				return w
			}
			var page struct {
				Repository string
				Format     string
				Root       browseNode
				NextCursor string
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
			if result.Repository == "" {
				result.Repository = page.Repository
				result.Format = page.Format
				result.Root = page.Root
				result.Root.Children = nil
			}
			result.Root.Children = append(result.Root.Children, page.Root.Children...)
			cursor = page.NextCursor
			if cursor == "" {
				break
			}
		}
		children := []browseNode{}
		for _, child := range result.Root.Children {
			if child.Kind == "folder" {
				w := walk(child.Path, depth+1)
				require.Equal(t, 200, w.Code)
				var nested browseTreeResp
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &nested))
				child.Children = nested.Root.Children
				if len(child.Children) == 0 {
					continue
				}
			}
			children = append(children, child)
		}
		result.Root.Children = children
		w := httptest.NewRecorder()
		w.WriteHeader(200)
		require.NoError(t, json.NewEncoder(w).Encode(result))
		return w
	}
	return walk("/", 0)
}
