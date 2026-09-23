//go:build integration

package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/system-design-evaluator/backend/internal/domain"
)

func TestPostgresRepositoryAppliesMigrationsAndPreservesDesignData(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository: %v", err)
	}
	t.Cleanup(repo.Close)

	owner, err := repo.CreateFirstAdmin(ctx, "Test Owner", "owner@stratum.test", "local-test-password")
	if err != nil {
		t.Fatalf("CreateFirstAdmin after migrations: %v", err)
	}
	reviewer, err := repo.CreateUser(ctx, "Test Reviewer", "reviewer@stratum.test", "reviewer", "")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	workspace, err := repo.CreateWorkspace(ctx, "Payments", "")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if _, err := repo.GrantWorkspaceAccess(ctx, domain.WorkspaceAccess{
		WorkspaceID: workspace.ID, UserID: owner.ID, CanRead: true, CanCreateDesign: true, CanManage: true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceAccess: %v", err)
	}
	group, err := repo.CreateAccessGroup(ctx, domain.AccessGroup{Name: "Reviewers", Description: "Architecture reviewers", OktaGroupName: "stratum-reviewers"})
	if err != nil {
		t.Fatalf("CreateAccessGroup: %v", err)
	}
	if _, err := repo.ReplaceAccessGroupMembers(ctx, group.ID, []string{reviewer.ID}); err != nil {
		t.Fatalf("ReplaceAccessGroupMembers: %v", err)
	}
	if _, err := repo.GrantWorkspaceGroupAccess(ctx, domain.WorkspaceGroupAccess{
		WorkspaceID: workspace.ID, GroupID: group.ID, CanRead: true,
	}); err != nil {
		t.Fatalf("GrantWorkspaceGroupAccess: %v", err)
	}

	document := []byte(`{
  "title": "Payments exact JSON",
  "components": [{"id":"api","name":"API  with  spaces"}],
  "connectors": [],
  "custom": {"preserve": "  whitespace inside values  "}
}`)
	design, err := repo.CreateDesign(ctx, workspace.ID, "Payment design", document, owner.ID)
	if err != nil {
		t.Fatalf("CreateDesign: %v", err)
	}
	loaded, err := repo.GetDesign(ctx, workspace.ID, design.ID)
	if err != nil {
		t.Fatalf("GetDesign: %v", err)
	}
	if !bytes.Equal(loaded.Document, document) {
		t.Fatalf("design JSON changed\n got: %s\nwant: %s", loaded.Document, document)
	}

	version, err := repo.CreateDesignVersion(ctx, workspace.ID, design.ID, owner.ID, "Ready for review")
	if err != nil {
		t.Fatalf("CreateDesignVersion: %v", err)
	}
	if version.VersionNumber != 1 || version.Status != "draft" || !bytes.Equal(version.Document, document) {
		t.Fatalf("version = %+v, want draft v1 preserving document", version)
	}
	doc, err := repo.CreateDesignDoc(ctx, workspace.ID, design.ID, "Decision log", "# Keep exact markdown\n\n  indented", "markdown")
	if err != nil {
		t.Fatalf("CreateDesignDoc: %v", err)
	}
	loadedDoc, err := repo.GetDesignDoc(ctx, workspace.ID, design.ID, doc.ID)
	if err != nil {
		t.Fatalf("GetDesignDoc: %v", err)
	}
	if loadedDoc.Body != doc.Body {
		t.Fatalf("doc body = %q, want %q", loadedDoc.Body, doc.Body)
	}
	conversation, err := repo.CreateAIConversation(ctx, domain.AIConversation{
		WorkspaceID: workspace.ID, DesignID: design.ID, Title: "Database review", AccessMode: "read_write", CreatedBy: owner.ID,
	})
	if err != nil {
		t.Fatalf("CreateAIConversation: %v", err)
	}
	conversation, err = repo.UpdateAIConversationAccess(ctx, workspace.ID, design.ID, conversation.ID, "read")
	if err != nil || conversation.AccessMode != "read" {
		t.Fatalf("UpdateAIConversationAccess conversation=%#v err=%v", conversation, err)
	}
	message, err := repo.CreateAIMessage(ctx, domain.AIMessage{
		ConversationID: conversation.ID, Role: "assistant", Content: "Review the database failover path.",
		References: []domain.AIMessageReference{{Kind: "component", ID: "api", Name: "API"}}, Provider: "google", Model: "gemini-test",
	})
	if err != nil {
		t.Fatalf("CreateAIMessage: %v", err)
	}
	conversations, err := repo.ListAIConversations(ctx, workspace.ID, design.ID)
	if err != nil || len(conversations) != 1 || conversations[0].ID != conversation.ID {
		t.Fatalf("ListAIConversations conversations=%#v err=%v", conversations, err)
	}
	loadedConversation, err := repo.GetAIConversation(ctx, workspace.ID, design.ID, conversation.ID)
	if err != nil || loadedConversation.AccessMode != "read" {
		t.Fatalf("GetAIConversation conversation=%#v err=%v", loadedConversation, err)
	}
	messages, err := repo.ListAIMessages(ctx, conversation.ID, 10)
	if err != nil || len(messages) != 1 || messages[0].ID != message.ID || len(messages[0].References) != 1 {
		t.Fatalf("ListAIMessages messages=%#v err=%v", messages, err)
	}
	status, err := InspectPostgresMigrations(ctx, repo.pool)
	if err != nil {
		t.Fatalf("InspectPostgresMigrations: %v", err)
	}
	if len(status.Pending) != 0 || len(status.Unknown) != 0 || len(status.Applied) == 0 {
		t.Fatalf("migration status after repository startup = %#v", status)
	}
	for _, migration := range status.Applied {
		if migration.Checksum == "" {
			t.Fatalf("migration %s has no checksum", migration.Version)
		}
	}
}

func TestManualPostgresMigrationWorkflowIsReadOnlyUntilUp(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer adminPool.Close()

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE")
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect isolated pool: %v", err)
	}
	defer pool.Close()

	status, err := InspectPostgresMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("inspect empty schema: %v", err)
	}
	files, err := loadPostgresMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Pending) != len(files) || len(status.Applied) != 0 {
		t.Fatalf("empty schema status = %#v", status)
	}
	var migrationTableExists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&migrationTableExists); err != nil {
		t.Fatal(err)
	}
	if migrationTableExists {
		t.Fatal("migration status inspection mutated the empty schema")
	}

	if err := ApplyPostgresMigrations(ctx, pool, DefaultMigrationOptions()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	status, err = InspectPostgresMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("inspect migrated schema: %v", err)
	}
	if len(status.Applied) != len(files) || len(status.Pending) != 0 || len(status.Unknown) != 0 {
		t.Fatalf("migrated schema status = %#v", status)
	}

	lockConnection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockConnection.Release()
	if _, err := lockConnection.Exec(ctx, `SELECT pg_advisory_lock($1)`, postgresMigrationLockKey); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = lockConnection.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, postgresMigrationLockKey)
	}()
	if err := ApplyPostgresMigrations(ctx, pool, MigrationOptions{LockTimeout: 100 * time.Millisecond}); err == nil {
		t.Fatal("concurrent migrator unexpectedly acquired the advisory lock")
	}
}

func TestMigrationStatusSupportsLegacyTrackingTable(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	schema := fmt.Sprintf("legacy_migration_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE") })

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO schema_migrations (version) VALUES ('001_init.sql');
`); err != nil {
		t.Fatal(err)
	}

	status, err := InspectPostgresMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("inspect legacy migration table: %v", err)
	}
	if len(status.Applied) != 1 || status.Applied[0].Version != "001_init.sql" || status.Applied[0].Checksum != "" {
		t.Fatalf("legacy migration status = %#v", status)
	}
}
