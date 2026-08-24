// Package k8s builds a Kubernetes client, auto-detecting in-cluster vs. local.
package k8s

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token" //nolint:gosec // path, not a credential

// Init returns an in-cluster client when running as a pod, otherwise a client
// built from --kubeconfig or ~/.kube/config.
func Init(cmd *cli.Command) (*kubernetes.Clientset, error) {
	if areWeInsideACluster() {
		return getInClusterClient()
	}

	return getOutClusterClient(cmd.String("kubeconfig"))
}

func areWeInsideACluster() bool {
	fi, err := os.Stat(serviceAccountTokenPath)

	return os.Getenv("KUBERNETES_SERVICE_HOST") != "" &&
		os.Getenv("KUBERNETES_SERVICE_PORT") != "" &&
		err == nil && !fi.IsDir()
}

func getInClusterClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func getOutClusterClient(k8sConfigPath string) (*kubernetes.Clientset, error) {
	configPath := k8sConfigPath

	if configPath == "" {
		home := homeDir()
		if home == "" {
			return nil, errors.New("k8s config can't be found")
		}

		configPath = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}

	return os.Getenv("USERPROFILE") // windows
}
