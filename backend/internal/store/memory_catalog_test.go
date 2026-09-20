package store

import (
	"context"
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/domain"
	"strings"
	"testing"
)

func TestMemoryRepositoryCatalogAssetsRejectDuplicateNormalizedNames(t *testing.T) {
	repo := NewMemoryRepository()

	created, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{
		Name:      " Purchase   Service ",
		Type:      "compute.service",
		CreatedBy: "user_test",
	})
	if err != nil {
		t.Fatalf("CreateCatalogAsset returned error: %v", err)
	}
	if created.NormalizedName != "purchase service" {
		t.Fatalf("normalized name = %q, want purchase service", created.NormalizedName)
	}

	if _, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{Name: "purchase service", Type: "compute.service"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate catalog asset error, got %v", err)
	}
}

func TestMemoryRepositoryCatalogAssetsPersistGovernanceMetadata(t *testing.T) {
	repo := NewMemoryRepository()

	created, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{
		Name:               "Legacy Payments API",
		Type:               "edge.gateway",
		Kind:               "system",
		Status:             "deprecated",
		Aliases:            []string{"Payments API", "legacy payments api", "Payments API"},
		ReplacementAssetID: "asset_replacement",
		UpdateMessage:      "Use Payments Platform v2 for new designs.",
		CreatedBy:          "user_test",
	})
	if err != nil {
		t.Fatalf("CreateCatalogAsset returned error: %v", err)
	}
	if created.Kind != "system" {
		t.Fatalf("kind = %q, want system", created.Kind)
	}
	if created.Status != "deprecated" {
		t.Fatalf("status = %q, want deprecated", created.Status)
	}
	if got, want := strings.Join(created.Aliases, ","), "Payments API,legacy payments api"; got != want {
		t.Fatalf("aliases = %q, want %q", got, want)
	}
	if created.ReplacementAssetID != "asset_replacement" {
		t.Fatalf("replacement asset id = %q", created.ReplacementAssetID)
	}
	if created.UpdateMessage == "" {
		t.Fatal("update message should be persisted")
	}

	updated, err := repo.UpdateCatalogAsset(context.Background(), created.ID, domain.CatalogAsset{
		Name:        "Legacy Payments API",
		Type:        "edge.gateway",
		Kind:        "invalid-kind",
		Status:      "invalid-status",
		Criticality: "high",
	})
	if err != nil {
		t.Fatalf("UpdateCatalogAsset returned error: %v", err)
	}
	if updated.Kind != "component" {
		t.Fatalf("invalid kind default = %q, want component", updated.Kind)
	}
	if updated.Status != "active" {
		t.Fatalf("invalid status default = %q, want active", updated.Status)
	}
}

func TestMemoryRepositoryCatalogAssetsTrackUsageAndBlockDelete(t *testing.T) {
	repo := NewMemoryRepository()
	workspace, err := repo.GetOrCreateGuestWorkspace(context.Background())
	if err != nil {
		t.Fatalf("GetOrCreateGuestWorkspace returned error: %v", err)
	}
	asset, err := repo.CreateCatalogAsset(context.Background(), domain.CatalogAsset{
		Name:      "Notification Service",
		Type:      "compute.service",
		CreatedBy: "user_test",
	})
	if err != nil {
		t.Fatalf("CreateCatalogAsset returned error: %v", err)
	}
	document := json.RawMessage(`{"components":[{"metadata":{"enterpriseAsset":{"assetId":"` + asset.ID + `"}}}]}`)
	if _, err := repo.CreateDesign(context.Background(), workspace.ID, "Uses Catalog", document, "user_test"); err != nil {
		t.Fatalf("CreateDesign returned error: %v", err)
	}

	assets, err := repo.ListCatalogAssets(context.Background(), "notification")
	if err != nil {
		t.Fatalf("ListCatalogAssets returned error: %v", err)
	}
	if len(assets) != 1 || assets[0].UsedInDesignCount != 1 {
		t.Fatalf("catalog assets = %#v, want one asset with one linked design", assets)
	}
	if err := repo.DeleteCatalogAsset(context.Background(), asset.ID); err == nil || !strings.Contains(err.Error(), "linked") {
		t.Fatalf("expected linked asset delete rejection, got %v", err)
	}
}
