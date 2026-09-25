package handlers

import (
	"reflect"
	"testing"
)

func TestSplitTags(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single (old behaviour)", "jawa", []string{"jawa"}},
		{"multi OR", "jawa,lucu", []string{"jawa", "lucu"}},
		{"whitespace and empties trimmed", " ui , ,design ", []string{"ui", "design"}},
		{"duplicates dropped", "ui,design,ui", []string{"ui", "design"}},
		{"commas only", ",,", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitTags(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitTags(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
