package httpapi

import (
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCatalogGovernanceMetadataRoundTripsThroughHTTP(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	createBody := `{
		"name":"Payments Platform",
		"kind":"system",
		"type":"compute.service",
		"status":"deprecated",
		"aliases":["Payments","payments"],
		"replacementAssetId":"asset_v2",
		"updateMessage":"Use Payments Platform v2."
	}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/catalog/assets", strings.NewReader(createBody))
	createRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	createRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create catalog asset status=%d body=%q", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Asset domain.CatalogAsset `json:"asset"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created catalog asset: %v", err)
	}
	if created.Asset.Kind != "system" || created.Asset.Status != "deprecated" || created.Asset.ReplacementAssetID != "asset_v2" {
		t.Fatalf("created governance metadata = %#v", created.Asset)
	}
	if got := strings.Join(created.Asset.Aliases, ","); got != "Payments" {
		t.Fatalf("created aliases=%q", got)
	}

	updateBody := `{
		"name":"Payments Platform",
		"kind":"system",
		"type":"compute.service",
		"status":"retired",
		"replacementAssetId":"asset_v3",
		"updateMessage":"Migrate before the next release."
	}`
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/admin/catalog/assets/"+created.Asset.ID, strings.NewReader(updateBody))
	updateRequest.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	updateRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update catalog asset status=%d body=%q", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated struct {
		Asset domain.CatalogAsset `json:"asset"`
	}
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated catalog asset: %v", err)
	}
	if updated.Asset.Status != "retired" || updated.Asset.ReplacementAssetID != "asset_v3" || updated.Asset.UpdateMessage == "" {
		t.Fatalf("updated governance metadata = %#v", updated.Asset)
	}
}
