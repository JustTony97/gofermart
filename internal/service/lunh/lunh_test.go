package lunh

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name   string
		number int64
		want   bool
	}{
		{
			name:   "valid short number",
			number: 79927398713,
			want:   true,
		},
		{
			name:   "valid order number from gofermart spec",
			number: 12345678903,
			want:   true,
		},
		{
			name:   "valid number with doubling overflow (>9)",
			number: 49927398716,
			want:   true,
		},
		{
			name:   "invalid checksum (off by one)",
			number: 79927398714,
			want:   false,
		},
		{
			name:   "invalid order number",
			number: 12345678901,
			want:   false,
		},
		{
			name:   "negative number",
			number: -79927398713,
			want:   false,
		},
		{
			name:   "single digit invalid",
			number: 5,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Valid(tt.number)
			if got != tt.want {
				t.Errorf("Valid(%d) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
