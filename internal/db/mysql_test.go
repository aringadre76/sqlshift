package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatMySQLDSN(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid standard url",
			input: "mysql://user:pass@localhost:3306/sqlshift",
			want:  "user:pass@tcp(localhost:3306)/sqlshift?multiStatements=true",
		},
		{
			name:    "missing port",
			input:   "mysql://user:pass@localhost/sqlshift",
			wantErr: true,
		},
		{
			name:    "query params unsupported",
			input:   "mysql://user:pass@localhost:3306/sqlshift?parseTime=true",
			wantErr: true,
		},
		{
			name:    "socket path unsupported",
			input:   "mysql://user:pass@/var/run/mysqld.sock/sqlshift",
			wantErr: true,
		},
		{
			name:    "empty host unsupported",
			input:   "mysql://user:pass@:3306/sqlshift",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatMySQLDSN(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
