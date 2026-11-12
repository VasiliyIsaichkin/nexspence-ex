package postgres

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
)

func (r *componentRepo) DockerNamesPage(ctx context.Context, repos []string, prefix, after string, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx, `WITH names AS (SELECT DISTINCT c.name FROM components c
 JOIN repositories r ON r.id=c.repository_id WHERE r.name=ANY($1)
 AND lower(trim(r.format)) IN ('docker','oci')
 AND ($2='' OR c.name=$2 OR starts_with(c.name,$2||'/'))
 ), branches AS (SELECT name, split_part(CASE WHEN $2='' THEN name
 WHEN name=$2 THEN '' ELSE substr(name,length($2)+2) END,'/',1) AS branch FROM names)
 SELECT name FROM branches WHERE (branch COLLATE "C",name COLLATE "C")>
 (split_part(CASE WHEN $2='' THEN $3 WHEN $3=$2 THEN '' ELSE substr($3,length($2)+2) END,'/',1) COLLATE "C",$3 COLLATE "C")
 ORDER BY branch COLLATE "C",name COLLATE "C" LIMIT $4`, repos, prefix, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (r *componentRepo) DockerRowsPage(ctx context.Context, repos []string, image, category, version, id string, limit int) ([]domain.DockerBrowseRow, error) {
	rows, err := r.db.Query(ctx, `WITH source AS (
 SELECT c.id,c.name,c.version,COALESCE(MIN(a.path),'') AS path,
 COALESCE(c.extra->>'oci_artifact_type','') AS artifact_type,
 COALESCE(c.extra->'oci_annotations'->>'dev.sigstore.bundle.predicateType','') AS predicate_type,
 array_position($1::text[],r.name) AS member_order
 FROM components c JOIN repositories r ON r.id=c.repository_id LEFT JOIN assets a ON a.component_id=c.id
 WHERE r.name=ANY($1) AND c.name=$2 AND lower(trim(r.format)) IN ('docker','oci') GROUP BY c.id,r.name
 ), leaves AS (SELECT DISTINCT ON (version) * FROM source
 WHERE (CASE WHEN strpos(path,'/blobs/')>0 THEN 'Blobs'
 WHEN version LIKE 'sha256:%' OR version LIKE 'sha512:%' THEN 'Manifests' ELSE 'Tags' END)=$3
 ORDER BY version,member_order,id::text COLLATE "C")
 SELECT id,name,version,path,artifact_type,predicate_type FROM leaves
 WHERE (version COLLATE "C",id::text COLLATE "C")>($4 COLLATE "C",$5 COLLATE "C")
 ORDER BY version COLLATE "C",id::text COLLATE "C" LIMIT $6`, repos, image, category, version, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DockerBrowseRow{}
	for rows.Next() {
		var row domain.DockerBrowseRow
		if err = rows.Scan(&row.ComponentID, &row.ImageName, &row.Version, &row.SamplePath, &row.ArtifactType, &row.PredicateType); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
