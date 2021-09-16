package autospotting

import (
	"testing"
)

func Test_min(t *testing.T) {
	tests := []struct {
		name string
		x    int
		y    int
		want int
	}{
		{
			name: "x<y",
			x:    2,
			y:    3,
			want: 2,
		},
		{
			name: "x>y",
			x:    3,
			y:    2,
			want: 2,
		},
		{
			name: "x=y",
			x:    3,
			y:    3,
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := min(tt.x, tt.y); got != tt.want {
				t.Errorf("min() = %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_max(t *testing.T) {
	tests := []struct {
		name string
		x    int
		y    int
		want int
	}{
		{
			name: "x<y",
			x:    2,
			y:    3,
			want: 3,
		},
		{
			name: "x>y",
			x:    3,
			y:    2,
			want: 3,
		},
		{
			name: "x=y",
			x:    3,
			y:    3,
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := max(tt.x, tt.y); got != tt.want {
				t.Errorf("max() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_itemInSlice(t *testing.T) {
	tests := []struct {
		name   string
		search string
		items  []string
		want   bool
	}{
		{
			name:   "item in slice",
			search: "b",
			items:  []string{"a", "b", "c"},
			want:   true,
		},
		{
			name:   "item not in slice",
			search: "d",
			items:  []string{"a", "b", "c"},
			want:   false,
		},
		{
			name:   "empty slice",
			search: "a",
			items:  []string{},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itemInSlice(tt.search, tt.items); got != tt.want {
				t.Errorf("itemInSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
