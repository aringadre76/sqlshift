package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// MigrationInfo represents migration data in a structured format
type MigrationInfo struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	State     string `json:"state"`
	AppliedAt string `json:"applied_at,omitempty"`
}

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// MigrationPlan represents a planned migration
type MigrationPlan struct {
	MigrationInfo `json:",inline"`
	UpSQL         string `json:"up_sql,omitempty"`
	DownSQL       string `json:"down_sql,omitempty"`
}

// PrintMigrations prints migrations in the specified format
func PrintMigrations(w io.Writer, format string, migrations []MigrationInfo) error {
	if format == "json" {
		return printJSON(w, migrations)
	}
	return printTable(w, migrations)
}

// PrintMigrationPlan prints a migration plan in the specified format
func PrintMigrationPlan(w io.Writer, format string, migrations []MigrationPlan) error {
	if format == "json" {
		return printJSON(w, migrations)
	}
	return printTablePlan(w, migrations)
}

func printJSON(w io.Writer, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func printTable(w io.Writer, migrations []MigrationInfo) error {
	writer := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(writer, "VERSION\tNAME\tSTATE\tAPPLIED_AT")
	for _, m := range migrations {
		appliedAt := m.AppliedAt
		if appliedAt == "" {
			appliedAt = "-"
		}
		fmt.Fprintf(writer, "%03d\t%s\t%s\t%s\n", m.Version, m.Name, m.State, appliedAt)
	}
	return writer.Flush()
}

func printTablePlan(w io.Writer, migrations []MigrationPlan) error {
	writer := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(writer, "VERSION\tNAME")
	for _, m := range migrations {
		fmt.Fprintf(writer, "%03d\t%s\n", m.Version, m.Name)
	}
	return writer.Flush()
}

// PrintStatus prints status information
func PrintStatus(w io.Writer, format string, migrations []MigrationInfo) error {
	if format == "json" {
		return printJSON(w, migrations)
	}
	return printTable(w, migrations)
}

// PrintValidate prints validation result
func PrintValidate(w io.Writer, format string, issues []ValidationIssue) error {
	if format == "json" {
		return printJSON(w, issues)
	}

	// Table format
	if len(issues) == 0 {
		fmt.Fprintln(w, "Validation OK.")
		return nil
	}

	writer := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(writer, "SEVERITY\tMESSAGE")
	for _, issue := range issues {
		fmt.Fprintf(writer, "%s\t%s\n", issue.Severity, issue.Message)
	}
	return writer.Flush()
}
