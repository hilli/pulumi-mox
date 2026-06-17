package provider

import "testing"

func TestValidateLogLevelArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    LogLevelArgs
		wantErr bool
	}{
		{name: "valid", args: LogLevelArgs{Package: "webadmin", Level: "debug"}},
		{name: "empty package", args: LogLevelArgs{Level: "debug"}, wantErr: true},
		{name: "empty level", args: LogLevelArgs{Package: "webadmin"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLogLevelArgs(tc.args)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
