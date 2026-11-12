//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/testutil/pgtest"
	"github.com/stretchr/testify/require"
)

func TestDockerBrowseDatabasePages(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	repos := NewRepositoryRepo(pool)
	repo := &domain.Repository{Name: "browse-page-fixture", Format: domain.FormatDocker, Type: domain.TypeHosted, Online: true}
	require.NoError(t, repos.Create(ctx, repo))
	comps := NewComponentRepo(pool)
	pager := comps
	for _, name := range []string{"team-b/a_b", "team-b/axb", "team-b/a_b2"} {
		for i := 0; i < 7; i++ {
			require.NoError(t, comps.Create(ctx, &domain.Component{RepositoryID: repo.ID, Repository: repo.Name, Format: "docker", Name: name, Version: fmt.Sprintf("1.0.%d", i)}))
		}
	}
	names, e := pager.DockerNamesPage(ctx, []string{repo.Name}, "team-b/a_b", "", 100)
	require.NoError(t, e)
	require.Equal(t, []string{"team-b/a_b"}, names)
	after, id := "", ""
	versions := []string{}
	for i := 0; i < 5; i++ {
		rows, e := pager.DockerRowsPage(ctx, []string{repo.Name}, "team-b/a_b", "Tags", after, id, 2)
		require.NoError(t, e)
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			versions = append(versions, r.Version)
		}
		last := rows[len(rows)-1]
		after, id = last.Version, last.ComponentID
	}
	require.Equal(t, []string{"1.0.0", "1.0.1", "1.0.2", "1.0.3", "1.0.4", "1.0.5", "1.0.6"}, versions)
	for _, name := range []string{"a-b", "a/c", "a.b", "a_b"} {
		require.NoError(t, comps.Create(ctx, &domain.Component{RepositoryID: repo.ID, Repository: repo.Name, Format: "docker", Name: name, Version: "1.0.0"}))
	}
	names, e = pager.DockerNamesPage(ctx, []string{repo.Name}, "", "", 2)
	require.NoError(t, e)
	require.Equal(t, []string{"a/c", "a-b"}, names)
	names, e = pager.DockerNamesPage(ctx, []string{repo.Name}, "", "a-b", 2)
	require.NoError(t, e)
	require.Equal(t, []string{"a.b", "a_b"}, names)
	second := &domain.Repository{Name: "browse-page-second", Format: domain.FormatDocker, Type: domain.TypeHosted, Online: true}
	require.NoError(t, repos.Create(ctx, second))
	require.NoError(t, comps.Create(ctx, &domain.Component{RepositoryID: second.ID, Repository: second.Name, Format: "docker", Name: "team-b/a_b", Version: "1.0.0"}))
	rows, e := pager.DockerRowsPage(ctx, []string{repo.Name, second.Name}, "team-b/a_b", "Tags", "", "", 8)
	require.NoError(t, e)
	require.Len(t, rows, 7)
}
