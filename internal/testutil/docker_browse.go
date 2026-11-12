package testutil

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"sort"
	"strings"
)

func (c *ComponentRepo) DockerNamesPage(ctx context.Context, repos []string, prefix, after string, limit int) ([]string, error) {
	rows, err := c.ListDockerBrowseRows(ctx, repos, 0)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	branch := func(name string) string {
		if name == prefix {
			return ""
		}
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix+"/")
		}
		return strings.SplitN(name, "/", 2)[0]
	}
	for _, r := range rows {
		if (prefix == "" || r.ImageName == prefix || strings.HasPrefix(r.ImageName, prefix+"/")) && (branch(r.ImageName) > branch(after) || (branch(r.ImageName) == branch(after) && r.ImageName > after)) {
			seen[r.ImageName] = true
		}
	}
	names := []string{}
	for name := range seen {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if branch(names[i]) == branch(names[j]) {
			return names[i] < names[j]
		}
		return branch(names[i]) < branch(names[j])
	})
	if len(names) > limit {
		names = names[:limit]
	}
	return names, nil
}
func (c *ComponentRepo) DockerRowsPage(ctx context.Context, repos []string, image, category, version, id string, limit int) ([]domain.DockerBrowseRow, error) {
	rows, err := c.ListDockerBrowseRows(ctx, repos, 0)
	if err != nil {
		return nil, err
	}
	out := []domain.DockerBrowseRow{}
	seen := map[string]bool{}
	for _, r := range rows {
		kind := "Tags"
		if strings.Contains(r.SamplePath, "/blobs/") {
			kind = "Blobs"
		} else if strings.HasPrefix(r.Version, "sha256:") || strings.HasPrefix(r.Version, "sha512:") {
			kind = "Manifests"
		}
		if r.ImageName == image && kind == category && !seen[r.Version] {
			seen[r.Version] = true
			if r.Version > version || (r.Version == version && r.ComponentID > id) {
				out = append(out, r)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Version == out[j].Version {
			return out[i].ComponentID < out[j].ComponentID
		}
		return out[i].Version < out[j].Version
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
