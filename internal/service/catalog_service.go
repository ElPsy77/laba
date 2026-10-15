// Package service is the service layer whose methods the aspects intercept.
package service

import (
	"fmt"
	"time"

	"springlab/pkg/ioc"
)

//go:generate go run springlab/cmd/aopgen -type catalogService -iface CatalogService -annotation Audited=springlab/internal/audit

// CatalogService — Listing 1. In Go a proxy cannot subclass a struct, so
// the bean is used through its interface (the JDK-proxy model).
type CatalogService interface {
	FindByID(id int64) (string, error)
	FindAll(limit int) ([]string, error)
	Remove(id int64) (string, error)
}

func init() {
	ioc.Component(newCatalogService)
}

type catalogService struct{}

func newCatalogService() *catalogService { return &catalogService{} }

func (s *catalogService) FindByID(id int64) (string, error) {
	sleep(50) // simulating a database call
	return fmt.Sprintf("Item no. %d", id), nil
}

// @Audited(action = "CATALOG_LIST", logArguments = true)
func (s *catalogService) FindAll(limit int) ([]string, error) {
	sleep(300) // a deliberately "slow" method
	items := make([]string, 0, limit)
	for i := 1; i <= limit; i++ {
		items = append(items, fmt.Sprintf("Item no. %d", i))
	}
	return items, nil
}

// @Audited(action = "CATALOG_REMOVE")
func (s *catalogService) Remove(id int64) (string, error) {
	if id <= 0 {
		return "", &IllegalArgumentError{fmt.Sprintf("Invalid identifier: %d", id)}
	}
	return fmt.Sprintf("Removed item no. %d", id), nil
}

func sleep(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

// IllegalArgumentError plays the role of IllegalArgumentException.
type IllegalArgumentError struct{ Message string }

func (e *IllegalArgumentError) Error() string { return e.Message }
