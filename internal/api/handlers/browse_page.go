package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/formats"
	"github.com/nexspence-oss/nexspence/internal/repository"
)

// DockerTree loads one branch, never the repository's complete layer tree.
func (h *BrowseHandler) DockerTree(c *gin.Context) {
	repoName := c.Param("name")
	ctx := c.Request.Context()
	repo, err := h.deps.Repos.Get(ctx, repoName)
	if err != nil || repo == nil {
		c.JSON(404, gin.H{"error": "repository not found"})
		return
	}
	if !repo.Format.IsOCIRegistry() {
		c.JSON(400, gin.H{"error": "repository is not an OCI registry format"})
		return
	}
	repos := []string{repoName}
	if repo.Type == domain.TypeGroup {
		repos = domain.GroupMemberNames(repo)
	}
	rawPath := c.DefaultQuery("path", "/")
	p, err := formats.CanonicalPath(rawPath)
	if err != nil || !strings.HasPrefix(rawPath, "/") {
		c.JSON(400, gin.H{"error": "invalid branch"})
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
	roles, _ := c.Get("roles")
	roleNames := stringSliceVal(roles)
	user := c.GetString("userID")
	binding := repoName + "\x00" + p + "\x00" + user
	cursor, err := h.decodeCursor(c.Query("cursor"), binding)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid or expired cursor"})
		return
	}
	pager, ok := h.deps.Components.(repository.DockerBrowsePager)
	if !ok {
		c.JSON(500, gin.H{"error": "browse paging unavailable"})
		return
	}
	root := &dockerBrowseNode{Kind: "folder", Label: "/", Path: p, Children: []*dockerBrowseNode{}}
	prefix := strings.Trim(p, "/")
	image, category := "", ""
	hasCategory := false
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		image, category = prefix[:i], prefix[i+1:]
		hasCategory = category == "Tags" || category == "Manifests" || category == "Blobs"
	}
	next := ""
	if hasCategory {
		// Authorize the image before selecting any of its component rows.
		if len(h.rbac.FilterOCIImageNames(ctx, user, roleNames, repo, []string{image})) != 0 {
			rows, e := pager.DockerRowsPage(ctx, repos, image, category, cursor.After, cursor.ID, limit+1)
			err = e
			if len(rows) > limit {
				rows = rows[:limit]
				last := rows[len(rows)-1]
				next = h.encodeCursor(browseCursor{last.Version, last.ComponentID}, binding)
			}
			for _, row := range rows {
				kind := "tag"
				if category == "Blobs" {
					kind = "blob"
				} else if category == "Manifests" {
					kind = "manifest"
				}
				root.Children = append(root.Children, &dockerBrowseNode{Kind: kind, Label: row.Version, Path: browseJoin(p, row.Version), ImageRef: row.ImageName, Version: row.Version, ComponentID: row.ComponentID, ArtifactType: ociArtifactLabel(row.ArtifactType, row.PredicateType)})
			}
		}
	} else {
		root.Children, err = h.browseFolders(ctx, pager, repos, repo, user, roleNames, prefix, cursor.After, limit+1)
		if len(root.Children) > limit {
			root.Children = root.Children[:limit]
			next = h.encodeCursor(browseCursor{After: root.Children[limit-1].Label}, binding)
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load repository branch"})
		return
	}
	c.JSON(200, gin.H{"repository": repoName, "format": string(repo.Format), "root": root, "nextCursor": next})
}

func (h *BrowseHandler) browseFolders(ctx context.Context, pager repository.DockerBrowsePager, repos []string, repo *domain.Repository, user string, roles []string, prefix, after string, limit int) ([]*dockerBrowseNode, error) {
	nodes := []*dockerBrowseNode{}
	seen := map[string]bool{}
	scan := ""
	// OCI image segments are lowercase ASCII. Uppercase category names precede them.
	if after != "" && after != "Blobs" && after != "Manifests" && after != "Tags" {
		scan = strings.TrimPrefix(prefix+"/"+after, "/")
	}
	for {
		names, err := pager.DockerNamesPage(ctx, repos, prefix, scan, 200)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return nodes, nil
		}
		allowed := h.rbac.FilterOCIImageNames(ctx, user, roles, repo, names)
		for _, name := range allowed {
			labels := []string{}
			if name == prefix {
				labels = []string{"Blobs", "Manifests", "Tags"}
			} else {
				tail := name
				if prefix != "" {
					tail = strings.TrimPrefix(name, prefix+"/")
				}
				labels = []string{strings.SplitN(tail, "/", 2)[0]}
			}
			for _, label := range labels {
				if label <= after || seen[label] {
					continue
				}
				seen[label] = true
				nodes = append(nodes, &dockerBrowseNode{Kind: "folder", Label: label, Path: browseJoin("/"+prefix, label), Lazy: true})
				if len(nodes) == limit {
					return nodes, nil
				}
			}
		}
		scan = names[len(names)-1]
		if len(names) < 200 {
			return nodes, nil
		}
	}
}
