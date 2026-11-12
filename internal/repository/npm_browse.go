package repository

import (
	"context"
	"github.com/nexspence-oss/nexspence/internal/domain"
)

// NPMPackage groups versions without loading component metadata or assets.
type NPMPackage struct {
	Name         string
	Repositories []string
}
type NPMBrowsePager interface {
	NPMNamesPage(context.Context, []string, string, int) ([]NPMPackage, error)
	NPMVersionsPage(context.Context, []string, string, string, int) ([]domain.Component, error)
}
