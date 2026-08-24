package grpcroute

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractHostnames(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GRPCRoute",
		"metadata":   map[string]any{"name": "grpc-web", "namespace": "default"},
		"spec": map[string]any{
			"hostnames": []any{"grpc.example.com"},
		},
	}}

	got := extractHostnames(obj)

	want := []string{"grpc.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractHostnames() = %v, want %v", got, want)
	}
}

func TestExtractHostnames_noHostnames(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GRPCRoute",
		"metadata":   map[string]any{"name": "grpc-web", "namespace": "default"},
		"spec":       map[string]any{},
	}}

	if got := extractHostnames(obj); got != nil {
		t.Errorf("extractHostnames() = %v, want nil", got)
	}
}

func TestSource(t *testing.T) {
	if source != "grpcroute" {
		t.Errorf("source = %q, want %q", source, "grpcroute")
	}
}
