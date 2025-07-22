package watcher

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Ingress struct {
	Host        string `json:"host"`
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
}

// GetIngresses scans the cluster and returns ingress metadata
func GetIngresses() []Ingress {
	var ingresses []Ingress

	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Printf("Error loading kubeconfig: %v\n", err)
		return ingresses
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error creating clientset: %v\n", err)
		return ingresses
	}

	allIngresses, err := clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing ingresses: %v\n", err)
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

// EncodeIngresses returns ingress data as JSON bytes
func EncodeIngresses(ings []Ingress) []byte {
	data, err := json.MarshalIndent(ings, "", "  ")
	if err != nil {
		log.Printf("❌ Marshal error: %v", err)
		return nil
	}
	return data
}

// StartWatcher triggers the callback on add/update/delete of any Ingress
func StartWatcher(onChange func([]Ingress)) {
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("load kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create clientset: %v", err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	ingressInformer := factory.Networking().V1().Ingresses().Informer()

	ingressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Println("🆕 Ingress added")
			onChange(GetIngresses())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			log.Println("🔄 Ingress updated")
			onChange(GetIngresses())
		},
		DeleteFunc: func(obj interface{}) {
			log.Println("❌ Ingress deleted")
			onChange(GetIngresses())
		},
	})

	stop := make(chan struct{})
	factory.Start(stop)
	factory.WaitForCacheSync(stop)
	<-stop // block forever
}
