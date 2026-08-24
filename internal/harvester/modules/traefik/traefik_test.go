package traefik

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseHostMatchers(t *testing.T) {
	tests := []struct {
		name  string
		match string
		want  []string
	}{
		{"single host", "Host(`example.com`)", []string{"example.com"}},
		{
			"multiple hosts in one Host()",
			"Host(`a.example.com`,`b.example.com`)",
			[]string{"a.example.com", "b.example.com"},
		},
		{
			"combined with other matchers",
			"Host(`example.com`) && PathPrefix(`/api`)",
			[]string{"example.com"},
		},
		{
			"two separate Host() calls joined by ||",
			"Host(`a.com`) || Host(`b.com`)",
			[]string{"a.com", "b.com"},
		},
		{"no Host() at all", "PathPrefix(`/healthz`)", nil},
		{"HostRegexp is not a literal domain, ignored", "HostRegexp(`^.+\\.example\\.com$`)", nil},
		{"empty match", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseHostMatchers(tt.match); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseHostMatchers(%q) = %v, want %v", tt.match, got, tt.want)
			}
		})
	}
}

func TestExtractHosts(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "IngressRoute",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
		},
		"spec": map[string]any{
			"routes": []any{
				map[string]any{
					"match": "Host(`example.com`) && PathPrefix(`/api`)",
					"kind":  "Rule",
				},
				map[string]any{
					"match": "Host(`other.example.com`)",
					"kind":  "Rule",
				},
			},
		},
	}}

	got := extractHosts(obj)
	sort.Strings(got)

	want := []string{"example.com", "other.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractHosts() = %v, want %v", got, want)
	}
}

func TestExtractHosts_noRoutes(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "IngressRoute",
		"metadata":   map[string]any{"name": "empty", "namespace": "default"},
	}}

	if got := extractHosts(obj); got != nil {
		t.Errorf("extractHosts() = %v, want nil", got)
	}
}
