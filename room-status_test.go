package goroom

import "testing"

func TestRoomStatusValues(t *testing.T) {
	// Test the constant values
	if Inactive != -1 {
		t.Errorf("Expected Inactive to be -1, got %d", Inactive)
	}
	if Open != 0 {
		t.Errorf("Expected Open to be 0, got %d", Open)
	}
	if Locked != 1 {
		t.Errorf("Expected Locked to be 1, got %d", Locked)
	}
}

func TestRoomStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   RoomStatus
		expected string
	}{
		{
			name:     "Inactive status",
			status:   Inactive,
			expected: "Inactive",
		},
		{
			name:     "Open status",
			status:   Open,
			expected: "Open",
		},
		{
			name:     "Locked status",
			status:   Locked,
			expected: "Locked",
		},
		{
			name:     "Unknown status",
			status:   RoomStatus(99),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("RoomStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
