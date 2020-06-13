package k8s

import (
	"errors"
	"github.com/urfave/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

func Init(c *cli.Context) (*kubernetes.Clientset, error) {
	if areWeInsideACluster() {
		return getInClusterClient()
	}

	return getOutClusterClient(c.String("kubeconfig"))
}

func areWeInsideACluster() bool {
	fi, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
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
	var configPath string

	if k8sConfigPath != "" {
		configPath = k8sConfigPath
	} else if home := homeDir(); home != "" {
		configPath = filepath.Join(home, ".kube", "config")
	} else {
		return nil, errors.New("k8s config can't be found")
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
