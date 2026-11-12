package postgres

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/repository"
)

func (r *componentRepo) NPMNamesPage(ctx context.Context, repos []string, after string, limit int) ([]repository.NPMPackage, error) {
	rows, err := r.db.Query(ctx, `SELECT c.name, array_agg(DISTINCT r.name)
 FROM components c JOIN repositories r ON r.id=c.repository_id
 WHERE r.name=ANY($1) AND r.format='npm' AND c.name COLLATE "C">$2 COLLATE "C"
 GROUP BY c.name ORDER BY c.name COLLATE "C" LIMIT $3`, repos, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repository.NPMPackage{}
	for rows.Next() {
		var p repository.NPMPackage
		if err = rows.Scan(&p.Name, &p.Repositories); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *componentRepo) NPMVersionsPage(ctx context.Context, repos []string, name, after string, limit int) ([]domain.Component, error) {
	rows, err := r.db.Query(ctx, `WITH versions AS (
 SELECT DISTINCT ON(c.version) c.id,c.name,c.version,r.name AS repository
 FROM components c JOIN repositories r ON r.id=c.repository_id
 WHERE r.name=ANY($1) AND r.format='npm' AND c.name=$2 AND c.version NOT IN ('','metadata') AND c.version COLLATE "C">$3 COLLATE "C"
 ORDER BY c.version,array_position($1::text[],r.name),c.id)
 SELECT id,name,version,repository FROM versions ORDER BY version COLLATE "C" LIMIT $4`, repos, name, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Component{}
	for rows.Next() {
		var c domain.Component
		if err = rows.Scan(&c.ID, &c.Name, &c.Version, &c.Repository); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
