package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nexspence-oss/nexspence/internal/api/handlers"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/formats"
	"github.com/nexspence-oss/nexspence/internal/repository"
	"github.com/nexspence-oss/nexspence/internal/service"
	"github.com/nexspence-oss/nexspence/internal/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"testing"
)

type npmPager struct {
	*testutil.ComponentRepo
	names        []repository.NPMPackage
	versionCalls int
}

func (p *npmPager) NPMNamesPage(_ context.Context, _ []string, after string, limit int) ([]repository.NPMPackage, error) {
	out := []repository.NPMPackage{}
	for _, n := range p.names {
		if n.Name > after {
			out = append(out, n)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
func (p *npmPager) NPMVersionsPage(_ context.Context, repos []string, name, after string, limit int) ([]domain.Component, error) {
	p.versionCalls++
	out := []domain.Component{}
	if len(repos) == 0 {
		return out, nil
	}
	for i := 0; i < 4; i++ {
		v := fmt.Sprintf("1.0.%d", i)
		if v > after {
			out = append(out, domain.Component{ID: v, Name: name, Version: v, Repository: repos[0]})
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

type npmGrants struct{ emptyRBACRepo }

func (npmGrants) GetUserPrivilegesWithSelectors(context.Context, string) ([]repository.PrivilegeWithSelector, error) {
	return []repository.PrivilegeWithSelector{{Actions: []string{"browse"}, Expression: `repository == "npm-hosted" && path.startsWith("/@team/")`}}, nil
}
func TestNPMBrowseLazyFilteredPaging(t *testing.T) {
	repos := testutil.NewRepoRepo()
	seedRepo(t, repos, &domain.Repository{ID: "npm", Name: "npm-hosted", Format: domain.FormatNPM, Type: domain.TypeHosted})
	comps := &npmPager{ComponentRepo: testutil.NewComponentRepo()}
	for i := 0; i < 205; i++ {
		comps.names = append(comps.names, repository.NPMPackage{Name: fmt.Sprintf("@aaa/p%03d", i), Repositories: []string{"npm-hosted"}})
	}
	for _, n := range []string{"@team/a", "@team/b", "@team/c"} {
		comps.names = append(comps.names, repository.NPMPackage{Name: n, Repositories: []string{"npm-hosted"}})
	}
	h := handlers.NewBrowseHandler(formats.Deps{Repos: repos, Components: comps}, service.NewRBACService(npmGrants{}, repos, zap.NewNop().Sugar(), false))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", "reader"); c.Next() })
	r.GET("/api/v1/browse/repositories/:name/npm-tree", h.NPMTree)
	base := "/api/v1/browse/repositories/npm-hosted/npm-tree"
	type item struct{ Name, Version, ComponentID string }
	var page struct {
		Items      []item
		NextCursor string
	}
	w := do(t, r, http.MethodGet, base+"?limit=2", nil)
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Equal(t, []item{{Name: "@team/a"}, {Name: "@team/b"}}, page.Items)
	require.NotEmpty(t, page.NextCursor)
	require.Zero(t, comps.versionCalls)
	rootCursor := page.NextCursor
	w = do(t, r, http.MethodGet, base+"?limit=2&cursor="+url.QueryEscape(rootCursor), nil)
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Equal(t, []item{{Name: "@team/c"}}, page.Items)
	require.Empty(t, page.NextCursor)
	w = do(t, r, http.MethodGet, base+"?name=@team/a&limit=2", nil)
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 2)
	require.Equal(t, "1.0.0", page.Items[0].Version)
	require.NotEmpty(t, page.NextCursor)
	w = do(t, r, http.MethodGet, base+"?name=@team/a&limit=2&cursor="+url.QueryEscape(page.NextCursor), nil)
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Equal(t, "1.0.2", page.Items[0].Version)
	require.Empty(t, page.NextCursor)
	w = do(t, r, http.MethodGet, base+"?name=@aaa/hidden", nil)
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Empty(t, page.Items)
	w = do(t, r, http.MethodGet, base+"?name=@team/a&cursor="+url.QueryEscape(rootCursor), nil)
	require.Equal(t, 400, w.Code)
	w = do(t, r, http.MethodGet, base+"?limit=501", nil)
	require.Equal(t, 400, w.Code)
}
