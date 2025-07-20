package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Ingress struct {
	Host        string
	ServiceName string
	ServicePort int
}

func GetIngresses() []Ingress {
	var ingresses []Ingress

	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err)
		return ingresses
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating clientset: %v\n", err)
		return ingresses
	}

	allIngresses, err := clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing ingresses: %v\n", err)
		return ingresses
	}

	for _, ing := range allIngresses.Items {
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				backend := path.Backend.Service
				ingresses = append(ingresses, Ingress{
					Host:        rule.Host,
					ServiceName: backend.Name,
					ServicePort: int(backend.Port.Number),
				})
			}
		}
	}

	return ingresses
}
