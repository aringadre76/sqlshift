package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	upMarker   = "-- shift:up"
	downMarker = "-- shift:down"
)

var fileNamePattern = regexp.MustCompile(`^(\d+)_(.+)\.sql$`)

type Migration struct {
	Version  int
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string
	FilePath string
}

func ParseFile(path string) (*Migration, error) {
	version, name, err := ParseFilename(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("parsing migration filename %s: %w", filepath.Base(path), err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading migration file %s: %w", path, err)
	}

	upSQL, downSQL, err := splitSQL(string(contents))
	if err != nil {
		return nil, fmt.Errorf("parsing migration file %s: %w", path, err)
	}

	return &Migration{
		Version:  version,
		Name:     name,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
		Checksum: Checksum(contents),
		FilePath: path,
	}, nil
}

func ParseFilename(name string) (int, string, error) {
	matches := fileNamePattern.FindStringSubmatch(name)
	if matches == nil {
		return 0, "", fmt.Errorf("invalid migration filename %s", name)
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", fmt.Errorf("invalid migration version %s: %w", matches[1], err)
	}
	if matches[2] == "" {
		return 0, "", fmt.Errorf("migration name cannot be empty")
	}

	return version, matches[2], nil
}

func LoadDir(dir string) ([]*Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory %s: %w", dir, err)
	}

	migrations := make([]*Migration, 0, len(entries))
	seenVersions := make(map[int]string)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		migrationPath := filepath.Join(dir, entry.Name())
		migration, parseErr := ParseFile(migrationPath)
		if parseErr != nil {
			return nil, parseErr
		}

		if existing, ok := seenVersions[migration.Version]; ok {
			return nil, fmt.Errorf("duplicate migration version %03d: %s and %s", migration.Version, existing, migrationPath)
		}
		seenVersions[migration.Version] = migrationPath
		migrations = append(migrations, migration)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func PlanUp(loaded []*Migration, appliedVersions map[int]struct{}) []*Migration {
	plan := make([]*Migration, 0, len(loaded))
	for _, migration := range loaded {
		if _, ok := appliedVersions[migration.Version]; ok {
			continue
		}
		plan = append(plan, migration)
	}

	return plan
}

func Checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func ValidateSequence(migrations []*Migration) error {
	if len(migrations) == 0 {
		return nil
	}

	sorted := append([]*Migration(nil), migrations...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	if sorted[0].Version != 1 {
		return fmt.Errorf("migration sequence gap: expected 001 but found %03d", sorted[0].Version)
	}

	for i := 1; i < len(sorted); i++ {
		if sorted[i].Version-sorted[i-1].Version != 1 {
			return fmt.Errorf("migration sequence gap: version %03d is followed by %03d", sorted[i-1].Version, sorted[i].Version)
		}
	}

	return nil
}

func splitSQL(contents string) (string, string, error) {
	upIndex := strings.Index(contents, upMarker)
	if upIndex == -1 {
		return "", "", fmt.Errorf("missing %s marker", upMarker)
	}

	afterUp := contents[upIndex+len(upMarker):]
	downIndex := strings.Index(afterUp, downMarker)

	if downIndex == -1 {
		upSQL := strings.TrimSpace(afterUp)
		if upSQL == "" {
			return "", "", fmt.Errorf("up section cannot be empty")
		}

		return upSQL, "", nil
	}

	upSQL := strings.TrimSpace(afterUp[:downIndex])
	downSQL := strings.TrimSpace(afterUp[downIndex+len(downMarker):])

	if upSQL == "" {
		return "", "", fmt.Errorf("up section cannot be empty")
	}

	return upSQL, downSQL, nil
}
