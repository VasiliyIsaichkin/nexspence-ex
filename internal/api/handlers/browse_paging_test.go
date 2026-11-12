package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDockerBrowseLazyPages(t *testing.T) {
	r, repos, comps, _, _, _ := mountBrowse(t)
	seedRepo(t, repos, &domain.Repository{ID: "d1", Name: "docker-host", Format: domain.FormatDocker, Type: domain.TypeHosted})
	rows := []domain.DockerBrowseRow{}
	for i := 0; i < 3100; i++ {
		rows = append(rows, domain.DockerBrowseRow{ComponentID: fmt.Sprint(i), ImageName: "team-a/first", Version: fmt.Sprint(i), SamplePath: "/blobs/team-a/first/" + fmt.Sprint(i)})
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, domain.DockerBrowseRow{ComponentID: fmt.Sprint(4000 + i), ImageName: fmt.Sprintf("team-b/image-%d", i), Version: "1.0.0", SamplePath: fmt.Sprintf("/manifests/team-b/image-%d/1.0.0", i)})
	}
	comps.DockerRowsByRepo = map[string][]domain.DockerBrowseRow{"docker-host": rows}
	base := "/api/v1/browse/repositories/docker-host/docker-tree"
	w := do(t, r, http.MethodGet, base+"?path=/", nil)
	require.Equal(t, 200, w.Code)
	var root struct {
		Root       browseNode
		NextCursor string
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &root))
	require.Len(t, root.Root.Children, 2)
	require.Empty(t, root.Root.Children[0].Children, "the root must not preload blob rows")
	require.Equal(t, "team-b", root.Root.Children[1].Label)
	cursor := ""
	seen := []string{}
	for i := 0; i < 5; i++ {
		w = do(t, r, http.MethodGet, base+"?path=/team-b&limit=2&cursor="+url.QueryEscape(cursor), nil)
		require.Equal(t, 200, w.Code)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &root))
		require.LessOrEqual(t, len(root.Root.Children), 2)
		for _, n := range root.Root.Children {
			seen = append(seen, n.Label)
		}
		cursor = root.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Equal(t, []string{"image-0", "image-1", "image-2", "image-3", "image-4"}, seen)
}

func TestDockerBrowsePagesUseBranchOrder(t *testing.T) {
	r, repos, comps, _, _, _ := mountBrowse(t)
	seedRepo(t, repos, &domain.Repository{ID: "d1", Name: "docker-host", Format: domain.FormatDocker, Type: domain.TypeHosted})
	rows := []domain.DockerBrowseRow{}
	for _, name := range []string{"a-b", "a/c", "a.b", "a_b", "z/end"} {
		rows = append(rows, domain.DockerBrowseRow{ComponentID: name, ImageName: name, Version: "1.0.0"})
	}
	comps.DockerRowsByRepo = map[string][]domain.DockerBrowseRow{"docker-host": rows}
	cursor := ""
	labels := []string{}
	for i := 0; i < 6; i++ {
		w := do(t, r, http.MethodGet, "/api/v1/browse/repositories/docker-host/docker-tree?limit=1&cursor="+url.QueryEscape(cursor), nil)
		require.Equal(t, 200, w.Code)
		var page struct {
			Root       browseNode
			NextCursor string
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		for _, n := range page.Root.Children {
			labels = append(labels, n.Label)
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Equal(t, []string{"a", "a-b", "a.b", "a_b", "z"}, labels)
}

func TestDockerBrowseGroupDeduplicatesBeforePaging(t *testing.T) {
	r, repos, comps, _, _, _ := mountBrowse(t)
	seedRepo(t, repos, &domain.Repository{ID: "g1", Name: "docker-group", Format: domain.FormatDocker, Type: domain.TypeGroup, FormatConfig: map[string]any{"member_names": []string{"first", "second"}}})
	comps.DockerRowsByRepo = map[string][]domain.DockerBrowseRow{
		"first":  {{ComponentID: "a", ImageName: "team-b/app", Version: "1.0.0"}, {ComponentID: "b", ImageName: "team-b/app", Version: "1.0.1"}},
		"second": {{ComponentID: "c", ImageName: "team-b/app", Version: "1.0.0"}, {ComponentID: "d", ImageName: "team-b/app", Version: "1.0.1"}},
	}
	w := do(t, r, http.MethodGet, "/api/v1/browse/repositories/docker-group/docker-tree?path=/team-b/app/Tags&limit=2", nil)
	require.Equal(t, 200, w.Code)
	var page struct {
		Root       browseNode
		NextCursor string
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Root.Children, 2)
	require.Equal(t, "1.0.0", page.Root.Children[0].Label)
	require.Equal(t, "1.0.1", page.Root.Children[1].Label)
	require.Empty(t, page.NextCursor)
}
