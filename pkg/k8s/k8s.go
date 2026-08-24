// Package k8s builds Kubernetes clients, auto-detecting in-cluster vs. local.
package k8s

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // path, not a credential

// Init returns a typed client: in-cluster when running as a pod, otherwise
// built from --kubeconfig or ~/.kube/config.
func Init(cmd *cli.Command) (*kubernetes.Clientset, error) {
	config, err := buildConfig(cmd)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

// InitDynamic returns a dynamic client against the same cluster as Init, for
// watching CRDs (Traefik IngressRoute, Gateway API HTTPRoute/GRPCRoute, ...)
// that have no generated typed client in this module.
func InitDynamic(cmd *cli.Command) (dynamic.Interface, error) {
	config, err := buildConfig(cmd)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func buildConfig(cmd *cli.Command) (*rest.Config, error) {
	if areWeInsideACluster() {
		return rest.InClusterConfig()
	}

	return getOutClusterConfig(cmd.String("kubeconfig"))
}

func areWeInsideACluster() bool {
	fi, err := os.Stat(serviceAccountTokenPath)

	return os.Getenv("KUBERNETES_SERVICE_HOST") != "" &&
		os.Getenv("KUBERNETES_SERVICE_PORT") != "" &&
		err == nil && !fi.IsDir()
}

func getOutClusterConfig(k8sConfigPath string) (*rest.Config, error) {
	configPath := k8sConfigPath

	if configPath == "" {
		home := homeDir()
		if home == "" {
			return nil, errors.New("k8s config can't be found")
		}

		configPath = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", configPath)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}

	return os.Getenv("USERPROFILE") // windows
}
