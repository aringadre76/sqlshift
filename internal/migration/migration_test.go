package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantVersion int
		wantName    string
		wantErr     bool
	}{
		{name: "valid", filename: "001_create_users.sql", wantVersion: 1, wantName: "create_users"},
		{name: "invalid no underscore", filename: "001create_users.sql", wantErr: true},
		{name: "invalid no extension", filename: "001_create_users", wantErr: true},
		{name: "invalid non numeric version", filename: "abc_create_users.sql", wantErr: true},
		{name: "invalid empty name", filename: "001_.sql", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, name, err := ParseFilename(tt.filename)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantVersion, version)
			require.Equal(t, tt.wantName, name)
		})
	}
}

func TestParseFile(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		m, err := ParseFile(fixturePath("happy_path", "001_create_users.sql"))
		require.NoError(t, err)
		require.Equal(t, 1, m.Version)
		require.Equal(t, "create_users", m.Name)
		require.Contains(t, m.UpSQL, "CREATE TABLE users")
		require.Contains(t, m.DownSQL, "DROP TABLE users")
		require.NotEmpty(t, m.Checksum)
	})

	t.Run("missing down is allowed", func(t *testing.T) {
		m, err := ParseFile(fixturePath("missing_down", "001_create_users.sql"))
		require.NoError(t, err)
		require.Empty(t, m.DownSQL)
		require.Contains(t, m.UpSQL, "CREATE TABLE users")
	})

	t.Run("missing up marker returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "001_bad.sql")
		err := os.WriteFile(path, []byte("CREATE TABLE nope (id INTEGER PRIMARY KEY);"), 0o644)
		require.NoError(t, err)

		_, parseErr := ParseFile(path)
		require.Error(t, parseErr)
	})
}

func TestChecksum(t *testing.T) {
	first := Checksum([]byte("hello"))
	second := Checksum([]byte("hello"))
	third := Checksum([]byte("world"))

	require.Equal(t, first, second)
	require.NotEqual(t, first, third)
}

func TestPlanUp(t *testing.T) {
	loaded := []*Migration{
		{Version: 1, Name: "one"},
		{Version: 2, Name: "two"},
		{Version: 3, Name: "three"},
	}
	applied := map[int]struct{}{
		1: {},
	}

	plan := PlanUp(loaded, applied)
	require.Len(t, plan, 2)
	require.Equal(t, 2, plan[0].Version)
	require.Equal(t, 3, plan[1].Version)
}

func TestLoadDir(t *testing.T) {
	t.Run("happy path fixtures", func(t *testing.T) {
		migrations, err := LoadDir(fixtureDir("happy_path"))
		require.NoError(t, err)
		require.Len(t, migrations, 3)
		require.Equal(t, 1, migrations[0].Version)
		require.Equal(t, 2, migrations[1].Version)
		require.Equal(t, 3, migrations[2].Version)
	})

	t.Run("gap fixtures still load", func(t *testing.T) {
		migrations, err := LoadDir(fixtureDir("gap_in_sequence"))
		require.NoError(t, err)
		require.Len(t, migrations, 3)
		require.Equal(t, 4, migrations[2].Version)
	})

	t.Run("duplicate versions return error", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "001_a.sql"), []byte(validSQL("a")), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "001_b.sql"), []byte(validSQL("b")), 0o644))

		_, err := LoadDir(dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate migration version 001")
	})
}

func TestValidateSequence(t *testing.T) {
	tests := []struct {
		name      string
		versions   []int
		wantErr   bool
	}{
		{name: "contiguous", versions: []int{1, 2, 3}, wantErr: false},
		{name: "gap in middle", versions: []int{1, 2, 4}, wantErr: true},
		{name: "starts after one", versions: []int{2, 3}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var migrations []*Migration
			for _, version := range tt.versions {
				migrations = append(migrations, &Migration{
					Version: version,
					Name:    "x",
				})
			}

			err := ValidateSequence(migrations)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func fixtureDir(name string) string {
	return filepath.Join("..", "..", "testdata", "migrations", name)
}

func fixturePath(dir string, file string) string {
	return filepath.Join(fixtureDir(dir), file)
}

func validSQL(name string) string {
	return "-- shift:up\nCREATE TABLE " + name + " (id INTEGER PRIMARY KEY);\n\n-- shift:down\nDROP TABLE " + name + ";"
}
