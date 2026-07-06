package architecture_test

import (
	"testing"

	"github.com/matthewmcnew/archtest"
)

var services = []string{
	"subber/services/notification-service",
	"subber/services/scanner-service",
	"subber/services/subscription-api",
}

func TestServiceModulesDoNotImportEachOther(t *testing.T) {
	for _, service := range services {
		archtest.Package(t, service+"/...").
			IncludeTests().
			ShouldNotDependDirectlyOn(otherServices(service)...)
	}
}

func TestSharedPackagesDoNotImportServiceOwnedCode(t *testing.T) {
	archtest.Package(t, "subber/pkg/...").
		IncludeTests().
		ShouldNotDependDirectlyOn(
			"subber/services/notification-service/...",
			"subber/services/scanner-service/...",
			"subber/services/subscription-api/...",
		)
}

func TestApplicationLayersDoNotDependOnAdaptersOrEntrypoints(t *testing.T) {
	t.Run("subscription application", func(t *testing.T) {
		archtest.Package(t, "subber/services/subscription-api/internal/subscription/...").
			IncludeTests().
			ShouldNotDependDirectlyOn(
				"subber/services/subscription-api/cmd/...",
				"subber/services/subscription-api/internal/config",
				"subber/services/subscription-api/internal/dbmigrations",
				"subber/services/subscription-api/internal/httpapi",
			)
	})

	t.Run("subscription saga", func(t *testing.T) {
		archtest.Package(t, "subber/services/subscription-api/internal/watchsaga/...").
			IncludeTests().
			ShouldNotDependDirectlyOn(
				"subber/services/subscription-api/cmd/...",
				"subber/services/subscription-api/internal/config",
				"subber/services/subscription-api/internal/dbmigrations",
				"subber/services/subscription-api/internal/httpapi",
			)
	})

	t.Run("scanner application", func(t *testing.T) {
		archtest.Package(t, "subber/services/scanner-service/internal/scanner/...").
			IncludeTests().
			ShouldNotDependDirectlyOn(
				"subber/services/scanner-service/cmd/...",
				"subber/services/scanner-service/internal/cache",
				"subber/services/scanner-service/internal/config",
				"subber/services/scanner-service/internal/dbmigrations",
				"subber/services/scanner-service/internal/github",
			)
	})

	t.Run("notification application", func(t *testing.T) {
		archtest.Package(t, "subber/services/notification-service/internal/delivery/...").
			IncludeTests().
			ShouldNotDependDirectlyOn(
				"subber/services/notification-service/cmd/...",
				"subber/services/notification-service/internal/config",
				"subber/services/notification-service/internal/dbmigrations",
				"subber/services/notification-service/internal/email",
				"subber/services/notification-service/internal/grpcapi",
			)
	})
}

func otherServices(service string) []string {
	other := make([]string, 0, len(services)-1)
	for _, candidate := range services {
		if candidate == service {
			continue
		}
		other = append(other, candidate+"/...")
	}
	return other
}
