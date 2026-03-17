package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	dbpkg "github.com/aringadre76/sqlshift/internal/db"
)

const (
	StatusApplied = "applied"
	StatusPending = "pending"

	SeverityError   = "error"
	SeverityWarning = "warning"
)

type Runner struct {
	DB            *sql.DB
	Dialect       dbpkg.Dialect
	MigrationsDir string
	TableName     string
}

type StatusEntry struct {
	Version   int
	Name      string
	State     string
	AppliedAt string
}

type ValidationIssue struct {
	Severity string
	Message  string
}

func NewRunner(database *sql.DB, dialect dbpkg.Dialect, dir string, tableName string) *Runner {
	return &Runner{
		DB:            database,
		Dialect:       dialect,
		MigrationsDir: dir,
		TableName:     tableName,
	}
}

func (r *Runner) Up(ctx context.Context) ([]Migration, error) {
	if err := r.Dialect.CreateHistoryTable(ctx, r.DB, r.TableName); err != nil {
		return nil, fmt.Errorf("creating history table: %w", err)
	}

	migrations, err := LoadDir(r.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("loading migrations from %s: %w", r.MigrationsDir, err)
	}

	applied, err := r.Dialect.GetApplied(ctx, r.DB, r.TableName)
	if err != nil {
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}

	byVersion := make(map[int]*Migration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}

	for _, record := range applied {
		migration, ok := byVersion[record.Version]
		if !ok {
			return nil, fmt.Errorf("migration %d is recorded as applied but file no longer exists on disk", record.Version)
		}
		if migration.Checksum != record.Checksum {
			return nil, fmt.Errorf(
				"checksum mismatch for migration %d (%s): file has been modified after it was applied -- run 'sqlshift validate' for details",
				record.Version,
				record.Name,
			)
		}
	}

	if err := ValidateSequence(migrations); err != nil {
		return nil, fmt.Errorf("validating migrations: %w", err)
	}

	appliedVersions := make(map[int]struct{}, len(applied))
	highestAppliedVersion := 0
	for _, record := range applied {
		appliedVersions[record.Version] = struct{}{}
		if record.Version > highestAppliedVersion {
			highestAppliedVersion = record.Version
		}
	}

	plan := PlanUp(migrations, appliedVersions)
	for _, migration := range plan {
		if migration.Version < highestAppliedVersion {
			return nil, fmt.Errorf(
				"migration %03d cannot be applied: version is lower than already-applied migration %03d",
				migration.Version,
				highestAppliedVersion,
			)
		}
	}

	appliedNow := make([]Migration, 0, len(plan))
	for _, migration := range plan {
		if err := r.applyMigration(ctx, migration); err != nil {
			return nil, err
		}
		appliedNow = append(appliedNow, *migration)
	}

	return appliedNow, nil
}

func (r *Runner) Down(ctx context.Context) (*Migration, error) {
	migrations, err := LoadDir(r.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("loading migrations from %s: %w", r.MigrationsDir, err)
	}

	applied, err := r.Dialect.GetApplied(ctx, r.DB, r.TableName)
	if err != nil {
		if r.Dialect.IsTableNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}
	if len(applied) == 0 {
		return nil, nil
	}

	last := applied[len(applied)-1]
	byVersion := make(map[int]*Migration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}

	migration, ok := byVersion[last.Version]
	if !ok {
		return nil, fmt.Errorf("migration %d is recorded as applied but file no longer exists on disk", last.Version)
	}
	if migration.DownSQL == "" {
		return nil, fmt.Errorf("migration %d (%s) has no down section and cannot be reverted", migration.Version, migration.Name)
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction for migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	if _, err := tx.ExecContext(ctx, migration.DownSQL); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("reverting migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if err := r.Dialect.DeleteMigration(ctx, tx, r.TableName, migration.Version); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("reverting migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	return migration, nil
}

func (r *Runner) Status(ctx context.Context) ([]StatusEntry, error) {
	migrations, err := LoadDir(r.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("loading migrations from %s: %w", r.MigrationsDir, err)
	}

	applied, err := r.Dialect.GetApplied(ctx, r.DB, r.TableName)
	if err != nil && !r.Dialect.IsTableNotFound(err) {
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}
	if err != nil && r.Dialect.IsTableNotFound(err) {
		applied = nil
	}

	appliedByVersion := make(map[int]dbpkg.AppliedMigration, len(applied))
	for _, record := range applied {
		appliedByVersion[record.Version] = record
	}

	entries := make([]StatusEntry, 0, len(migrations)+len(applied))
	seen := make(map[int]struct{}, len(migrations))
	for _, migration := range migrations {
		entry := StatusEntry{
			Version: migration.Version,
			Name:    migration.Name,
			State:   StatusPending,
		}
		if record, ok := appliedByVersion[migration.Version]; ok {
			entry.State = StatusApplied
			entry.AppliedAt = record.AppliedAt
		}
		entries = append(entries, entry)
		seen[migration.Version] = struct{}{}
	}

	for _, record := range applied {
		if _, ok := seen[record.Version]; ok {
			continue
		}
		entries = append(entries, StatusEntry{
			Version:   record.Version,
			Name:      record.Name,
			State:     StatusApplied,
			AppliedAt: record.AppliedAt,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version < entries[j].Version
	})

	return entries, nil
}

func (r *Runner) Validate(ctx context.Context) ([]ValidationIssue, error) {
	migrations, err := LoadDir(r.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("loading migrations from %s: %w", r.MigrationsDir, err)
	}

	issues := make([]ValidationIssue, 0)
	if err := ValidateSequence(migrations); err != nil {
		issues = append(issues, ValidationIssue{
			Severity: SeverityError,
			Message:  err.Error(),
		})
	}
	for _, migration := range migrations {
		if migration.DownSQL == "" {
			issues = append(issues, ValidationIssue{
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("migration %03d (%s) is missing down section", migration.Version, migration.Name),
			})
		}
	}

	applied, err := r.Dialect.GetApplied(ctx, r.DB, r.TableName)
	if err != nil {
		if r.Dialect.IsTableNotFound(err) {
			return issues, nil
		}
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}

	byVersion := make(map[int]*Migration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}

	for _, record := range applied {
		migration, ok := byVersion[record.Version]
		if !ok {
			issues = append(issues, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("migration %03d is recorded as applied but file no longer exists on disk", record.Version),
			})
			continue
		}
		if migration.Checksum != record.Checksum {
			issues = append(issues, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("checksum mismatch for migration %d (%s)", record.Version, record.Name),
			})
		}
	}

	return issues, nil
}

func HasValidationErrors(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			return true
		}
	}

	return false
}

func (r *Runner) applyMigration(ctx context.Context, migration *Migration) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction for migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	start := time.Now()
	if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("applying migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if err := r.Dialect.InsertMigration(ctx, tx, r.TableName, dbpkg.AppliedMigration{
		Version:         migration.Version,
		Name:            migration.Name,
		Checksum:        migration.Checksum,
		ExecutionTimeMs: int(time.Since(start).Milliseconds()),
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("applying migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	return nil
}
