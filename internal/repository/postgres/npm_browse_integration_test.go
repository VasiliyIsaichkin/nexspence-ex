//go:build integration

package postgres

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/testutil/pgtest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNPMBrowseDatabasePages(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	repos := NewRepositoryRepo(pool)
	comps := NewComponentRepo(pool)
	names := []string{"npm-browse-first", "npm-browse-second"}
	for _, n := range names {
		r := &domain.Repository{Name: n, Format: domain.FormatNPM, Type: domain.TypeHosted, Online: true}
		require.NoError(t, repos.Create(ctx, r))
		for _, pkg := range []string{"@team/a_b", "@team/axb", "plain"} {
			for _, v := range []string{"1.0.0", "1.0.1", "metadata"} {
				require.NoError(t, comps.Create(ctx, &domain.Component{RepositoryID: r.ID, Repository: n, Format: "npm", Name: pkg, Version: v}))
			}
		}
	}
	rows, e := comps.NPMNamesPage(ctx, names, "", 2)
	require.NoError(t, e)
	require.Len(t, rows, 2)
	require.Equal(t, "@team/a_b", rows[0].Name)
	require.Len(t, rows[0].Repositories, 2)
	rows, e = comps.NPMNamesPage(ctx, names, rows[1].Name, 2)
	require.NoError(t, e)
	require.Len(t, rows, 1)
	require.Equal(t, "plain", rows[0].Name)
	versions, e := comps.NPMVersionsPage(ctx, names, "@team/a_b", "", 1)
	require.NoError(t, e)
	require.Len(t, versions, 1)
	require.Equal(t, "1.0.0", versions[0].Version)
	require.Equal(t, names[0], versions[0].Repository)
	require.Empty(t, versions[0].Assets)
	versions, e = comps.NPMVersionsPage(ctx, names, "@team/a_b", "1.0.0", 10)
	require.NoError(t, e)
	require.Len(t, versions, 1)
	require.Equal(t, "1.0.1", versions[0].Version)
	versions, e = comps.NPMVersionsPage(ctx, []string{}, "@team/a_b", "", 10)
	require.NoError(t, e)
	require.Empty(t, versions)
}
