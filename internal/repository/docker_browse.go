package repository

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
)

// DockerBrowsePager bounds database reads independently of the caller's visible page.
type DockerBrowsePager interface {
	DockerNamesPage(context.Context, []string, string, string, int) ([]string, error)
	DockerRowsPage(context.Context, []string, string, string, string, string, int) ([]domain.DockerBrowseRow, error)
}
