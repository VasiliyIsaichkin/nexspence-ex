package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/formats"
	"github.com/nexspence-oss/nexspence/internal/repository"
)

type npmBrowseItem struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	ComponentID string `json:"componentId,omitempty"`
	Repository  string `json:"repository,omitempty"`
}

// NPMTree returns names or the versions of one package. Assets are loaded
// through the existing component-detail endpoint and its permissions.
func (h *BrowseHandler) NPMTree(c *gin.Context) {
	ctx := c.Request.Context()
	repoName := c.Param("name")
	name := c.Query("name")
	repo, err := h.deps.Repos.Get(ctx, repoName)
	if err != nil || repo == nil {
		c.JSON(404, gin.H{"error": "repository not found"})
		return
	}
	if repo.Format != domain.FormatNPM {
		c.JSON(400, gin.H{"error": "repository is not npm"})
		return
	}
	if _, err := formats.CanonicalPath(name); err != nil || len(name) > 214 {
		c.JSON(400, gin.H{"error": "invalid package name"})
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 500 {
			c.JSON(400, gin.H{"error": "limit must be 1..500"})
			return
		}
		limit = n
	}
	repos, err := expandGroupMemberRepoNames(ctx, h.deps.Repos, repoName)
	if err != nil {
		c.JSON(500, gin.H{"error": "could not load repository"})
		return
	}
	anon := map[string]bool{}
	for _, n := range repos {
		r, e := h.deps.Repos.Get(ctx, n)
		if e != nil {
			c.JSON(500, gin.H{"error": "could not load repository"})
			return
		}
		if r != nil {
			anon[n] = r.AllowAnonymous
		}
	}
	user := c.GetString("userID")
	roles, _ := c.Get("roles")
	allowedRepos := func(pkg string, candidates []string) []string {
		comps := make([]domain.Component, 0, len(candidates))
		for _, n := range candidates {
			comps = append(comps, domain.Component{Name: pkg, Repository: n})
		}
		allowed := h.rbac.FilterComponents(ctx, user, stringSliceVal(roles), comps, anon)
		out := []string{}
		for _, a := range allowed {
			out = append(out, a.Repository)
		}
		return out
	}
	binding := "npm\x00" + repoName + "\x00" + name + "\x00" + user
	cursor, err := h.decodeCursor(c.Query("cursor"), binding)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid or expired cursor"})
		return
	}
	pager, ok := h.deps.Components.(repository.NPMBrowsePager)
	if !ok {
		c.JSON(500, gin.H{"error": "browse paging unavailable"})
		return
	}
	items := []npmBrowseItem{}
	next := ""
	if name != "" {
		rows, e := pager.NPMVersionsPage(ctx, allowedRepos(name, repos), name, cursor.After, limit+1)
		err = e
		if len(rows) > limit {
			rows = rows[:limit]
			next = h.encodeCursor(browseCursor{After: rows[limit-1].Version}, binding)
		}
		for _, r := range rows {
			items = append(items, npmBrowseItem{Name: r.Name, Version: r.Version, ComponentID: r.ID, Repository: r.Repository})
		}
	} else {
		after := cursor.After
		// Bound each scan, including a reader who cannot see most package names.
		for scanned := 0; scanned < maxFilterScanPages; scanned++ {
			rows, e := pager.NPMNamesPage(ctx, repos, after, 200)
			if e != nil {
				err = e
				break
			}
			comps := make([]domain.Component, 0, len(rows))
			for _, r := range rows {
				for _, member := range r.Repositories {
					comps = append(comps, domain.Component{Name: r.Name, Repository: member})
				}
			}
			visible := map[string]bool{}
			for _, comp := range h.rbac.FilterComponents(ctx, user, stringSliceVal(roles), comps, anon) {
				visible[comp.Name] = true
			}
			for _, r := range rows {
				if !visible[r.Name] {
					after = r.Name
					continue
				}
				if len(items) == limit {
					next = h.encodeCursor(browseCursor{After: after}, binding)
					break
				}
				items = append(items, npmBrowseItem{Name: r.Name})
				after = r.Name
			}
			if next != "" || len(rows) < 200 {
				break
			}
			if scanned == maxFilterScanPages-1 {
				next = h.encodeCursor(browseCursor{After: after}, binding)
			}
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load npm branch"})
		return
	}
	c.JSON(200, gin.H{"items": items, "nextCursor": next})
}
