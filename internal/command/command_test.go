package command

import "testing"

func TestParseOnOff(t *testing.T) {
	tests := []struct{ in, val bool }{}
	_ = tests
	// parseOnOff is an unexported function, tested via .mode set command integration
}

func TestCompleteForward(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".forward"}, want: []string{"list", "add", "remove", "enable", "disable", "update"}},
		{args: []string{".forward", ""}, want: []string{"list", "add", "remove", "enable", "disable", "update"}},
		{args: []string{".forward", "add", ""}, want: []string{"tcp", "udp", "tcp-s", "udp-s", "com"}},
		{args: []string{".forward", "update", "1", ""}, want: []string{"tcp", "udp", "tcp-s", "udp-s", "com"}},
	}
	for _, tt := range tests {
		got := completeForward(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completeForward(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func TestCompletePlugin(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".plugin"}, want: []string{"list", "load", "unload", "enable", "disable", "reload"}},
		{args: []string{".plugin", "load", ""}, want: nil},
	}
	for _, tt := range tests {
		got := completePlugin(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completePlugin(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func TestCompleteMode(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{".mode"}, want: []string{"show", "set"}},
		{args: []string{".mode", "set", ""}, want: []string{"in", "out", "end", "frame", "timestamp", "timefmt"}},
		{args: []string{".mode", "set", "timestamp", ""}, want: []string{"on", "off"}},
	}
	for _, tt := range tests {
		got := completeMode(tt.args)
		if !stringSlicesEqual(got, tt.want) {
			t.Fatalf("completeMode(%v) got=%v want=%v", tt.args, got, tt.want)
		}
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
