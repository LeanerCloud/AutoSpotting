package autospotting

import (
	"reflect"
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
func TestRemoveDuplicateStr(t *testing.T) {
	// Test cases
	testCases := []struct {
		name   string
		input  []*string
		output []*string
	}{
		{
			name:   "Remove duplicates from input with multiple duplicates",
			input:  []*string{newString("foo"), newString("bar"), newString("baz"), newString("foo"), newString("qux"), newString("baz")},
			output: []*string{newString("foo"), newString("bar"), newString("baz"), newString("qux")},
		},
		{
			name:   "Remove duplicates from input with some duplicates",
			input:  []*string{newString("hello"), newString("world"), newString("hello"), newString("gopher"), newString("world"), newString("gopher")},
			output: []*string{newString("hello"), newString("world"), newString("gopher")},
		},
		{
			name:   "Input with no duplicates",
			input:  []*string{newString("a"), newString("b"), newString("c")},
			output: []*string{newString("a"), newString("b"), newString("c")},
		},
		{
			name:   "Empty input slice",
			input:  []*string{},
			output: []*string{},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := removeDuplicateStr(tc.input)

			if !reflect.DeepEqual(output, tc.output) {
				t.Errorf("Expected %v but got %v", tc.output, output)
			}
		})
	}
}

func newString(s string) *string {
	return &s
}
