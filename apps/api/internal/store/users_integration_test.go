package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/db"
)

const testDatabaseTimeout = 5 * time.Second

var testDatabasePool *pgxpool.Pool

func TestMain(m *testing.M) {
	testDatabaseURL := strings.TrimSpace(
		os.Getenv("TEST_DATABASE_URL"),
	)

	// Keep ordinary unit tests runnable when PostgreSQL integration
	// testing has not been configured.
	if testDatabaseURL == "" {
		os.Exit(m.Run())
	}

	migrator, err := newTestMigrator(testDatabaseURL)
	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			"initialize test migrations:",
			err,
		)

		os.Exit(1)
	}

	// Always prove that the complete schema can be rebuilt from zero.
	if err := migrator.Down(); err != nil &&
		!errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(
			os.Stderr,
			"reset test database migrations:",
			err,
		)

		closeMigrator(migrator)
		os.Exit(1)
	}

	if err := migrator.Up(); err != nil &&
		!errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(
			os.Stderr,
			"apply test database migrations:",
			err,
		)

		closeMigrator(migrator)
		os.Exit(1)
	}

	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)

	testDatabasePool, err = db.New(
		databaseContext,
		testDatabaseURL,
	)

	cancelDatabase()

	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			"open test database:",
			err,
		)

		closeMigrator(migrator)
		os.Exit(1)
	}

	exitCode := m.Run()

	testDatabasePool.Close()

	// Leave the dedicated test database at migration version zero.
	if err := migrator.Down(); err != nil &&
		!errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(
			os.Stderr,
			"roll back test database migrations:",
			err,
		)

		exitCode = 1
	}

	if !closeMigrator(migrator) {
		exitCode = 1
	}

	os.Exit(exitCode)
}

func TestUserStoreCreate(t *testing.T) {
	userStore := newIntegrationTestUserStore(t)

	user := &User{
		Email:             "create-user@example.com",
		PasswordHash:      "dummy-encoded-password-hash",
		PasswordAlgorithm: "test",
	}

	err := userStore.Create(
		context.Background(),
		user,
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if user.ID == "" {
		t.Error("expected database-generated user ID")
	}

	if user.Status != "active" {
		t.Errorf(
			"expected status active, got %q",
			user.Status,
		)
	}

	if user.CreatedAt.IsZero() {
		t.Error("expected created timestamp")
	}

	if user.UpdatedAt.IsZero() {
		t.Error("expected updated timestamp")
	}

	if user.UpdatedAt.Before(user.CreatedAt) {
		t.Error("expected updated timestamp not to precede creation")
	}
}

func TestUserStoreGetByEmail(t *testing.T) {
	userStore := newIntegrationTestUserStore(t)

	createdUser := &User{
		Email:             "lookup-user@example.com",
		PasswordHash:      "dummy-lookup-password-hash",
		PasswordAlgorithm: "test",
	}

	if err := userStore.Create(
		context.Background(),
		createdUser,
	); err != nil {
		t.Fatalf("create user: %v", err)
	}

	storedUser, err := userStore.GetByEmail(
		context.Background(),
		createdUser.Email,
	)
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}

	if storedUser.ID != createdUser.ID {
		t.Errorf(
			"expected ID %q, got %q",
			createdUser.ID,
			storedUser.ID,
		)
	}

	if storedUser.Email != createdUser.Email {
		t.Errorf(
			"expected email %q, got %q",
			createdUser.Email,
			storedUser.Email,
		)
	}

	if storedUser.PasswordHash != createdUser.PasswordHash {
		t.Error("password hash did not round trip")
	}

	if storedUser.PasswordAlgorithm != createdUser.PasswordAlgorithm {
		t.Errorf(
			"expected password algorithm %q, got %q",
			createdUser.PasswordAlgorithm,
			storedUser.PasswordAlgorithm,
		)
	}

	if storedUser.Status != "active" {
		t.Errorf(
			"expected status active, got %q",
			storedUser.Status,
		)
	}
}

func TestUserStoreGetByEmailReturnsDisabledUser(
	t *testing.T,
) {
	userStore := newIntegrationTestUserStore(t)

	user := &User{
		Email:             "disabled-user@example.com",
		PasswordHash:      "dummy-disabled-password-hash",
		PasswordAlgorithm: "test",
	}

	if err := userStore.Create(
		context.Background(),
		user,
	); err != nil {
		t.Fatalf("create user: %v", err)
	}

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE users
			SET
				status = 'disabled',
				updated_at = now()
			WHERE id = $1::uuid
		`,
		user.ID,
	)
	if err != nil {
		t.Fatalf("disable user: %v", err)
	}

	storedUser, err := userStore.GetByEmail(
		context.Background(),
		user.Email,
	)
	if err != nil {
		t.Fatalf("get disabled user: %v", err)
	}

	if storedUser.Status != "disabled" {
		t.Errorf(
			"expected status disabled, got %q",
			storedUser.Status,
		)
	}
}

func TestUserStoreGetByEmailReturnsNotFound(
	t *testing.T,
) {
	userStore := newIntegrationTestUserStore(t)

	user, err := userStore.GetByEmail(
		context.Background(),
		"missing-user@example.com",
	)

	if user != nil {
		t.Fatal("expected no user")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"expected ErrNotFound, got %v",
			err,
		)
	}
}

func TestUserStoreCreateReturnsDuplicateEmail(
	t *testing.T,
) {
	userStore := newIntegrationTestUserStore(t)

	firstUser := &User{
		Email:             "duplicate-user@example.com",
		PasswordHash:      "first-dummy-hash",
		PasswordAlgorithm: "test",
	}

	if err := userStore.Create(
		context.Background(),
		firstUser,
	); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	secondUser := &User{
		Email:             firstUser.Email,
		PasswordHash:      "second-dummy-hash",
		PasswordAlgorithm: "test",
	}

	err := userStore.Create(
		context.Background(),
		secondUser,
	)

	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf(
			"expected ErrDuplicateEmail, got %v",
			err,
		)
	}

	if strings.Contains(
		err.Error(),
		"users_email_unique",
	) {
		t.Fatal("store error exposed database constraint details")
	}

	if strings.Contains(
		err.Error(),
		"duplicate key",
	) {
		t.Fatal("store error exposed raw PostgreSQL details")
	}
}

func newIntegrationTestUserStore(
	t *testing.T,
) *UserStore {
	t.Helper()

	if testDatabasePool == nil {
		t.Skip(
			"TEST_DATABASE_URL is not configured",
		)
	}

	resetIntegrationTestTables(t)

	return NewUserStore(testDatabasePool)
}

func resetIntegrationTestTables(t *testing.T) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			TRUNCATE TABLE
				item_versions,
				vault_items,
				vaults,
				sessions,
				users
			CASCADE
		`,
	)
	if err != nil {
		t.Fatalf(
			"reset integration test tables: %v",
			err,
		)
	}
}

func newTestMigrator(
	databaseURL string,
) (*migrate.Migrate, error) {
	migrationSource, err := testMigrationSource()
	if err != nil {
		return nil, err
	}

	migrator, err := migrate.New(
		migrationSource,
		databaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create migrator: %w",
			err,
		)
	}

	return migrator, nil
}

func testMigrationSource() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New(
			"locate integration test file",
		)
	}

	migrationsPath := filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"migrations",
	)

	absolutePath, err := filepath.Abs(
		migrationsPath,
	)
	if err != nil {
		return "", fmt.Errorf(
			"resolve migrations path: %w",
			err,
		)
	}

	sourceURL := &url.URL{
		Scheme: "file",
		Path:   absolutePath,
	}

	return sourceURL.String(), nil
}

func closeMigrator(
	migrator *migrate.Migrate,
) bool {
	sourceError, databaseError := migrator.Close()

	if sourceError != nil {
		fmt.Fprintln(
			os.Stderr,
			"close migration source:",
			sourceError,
		)

		return false
	}

	if databaseError != nil {
		fmt.Fprintln(
			os.Stderr,
			"close migration database:",
			databaseError,
		)

		return false
	}

	return true
}
