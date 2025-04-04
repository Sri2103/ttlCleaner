package main

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	// flag.Parse()

	// config, err := rest.InClusterConfig()
	// if err != nil {
	// 	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "Error creating Kubernetes config: %v\n", err)
	// 		os.Exit(1)
	// 	}
	// }

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes config: %v\n", err)
		os.Exit(1)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dynamic client: %v\n", err)
		os.Exit(1)
	}

	watchResourceTTL(config, dynClient)
}

func watchResourceTTL(config *rest.Config, dynClient dynamic.Interface) {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)

	gvr := schema.GroupVersionResource{
		Group:    "cleanup.example.com",
		Version:  "v1",
		Resource: "resourcettls",
	}

	informer := factory.ForResource(gvr).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			processUnstructuredResourceTTL(dynClient, config, u)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newU := newObj.(*unstructured.Unstructured)
			processUnstructuredResourceTTL(dynClient, config, newU)
		},
		DeleteFunc: func(obj interface{}) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if ok {
					u, ok = tombstone.Obj.(*unstructured.Unstructured)
				}
			}
			if ok {
				fmt.Printf("ResourceTTL deleted: %s\n", u.GetName())
				// Optional: Cancel associated tasks or cleanup logic
			}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	informer.Run(stopCh)
}

func processUnstructuredResourceTTL(dynClient dynamic.Interface, config *rest.Config, obj *unstructured.Unstructured) {
	fmt.Printf("Found ResourceTTL: %s\n", obj.GetName())

	spec := obj.Object["spec"].(map[string]interface{})
	kind := spec["resourceKind"].(string)
	name, _ := spec["resourceName"].(string)
	namespace := spec["namespace"].(string)
	ttlSeconds := int64(spec["ttlSeconds"].(int64))

	annotations := map[string]string{}
	if raw, ok := spec["matchAnnotations"].(map[string]interface{}); ok {
		for k, v := range raw {
			annotations[k] = v.(string)
		}
	}

	switch kind {
	case "Pod":
		deleteExpiredPods(dynClient, namespace, name, ttlSeconds, annotations)

	case "Deployment":
		deleteExpiredDeployment(dynClient, namespace, name, ttlSeconds, annotations)
	default:
		fmt.Printf("Unsupported resource kind: %s\n", kind)
	}
}

func deleteExpiredPods(client dynamic.Interface, namespace, name string, ttl int64, matchAnnotations map[string]string) {
	Podclient := client.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}).Namespace(namespace)

	pods, err := Podclient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to list pods: %v\n", err)
		return
	}

	now := time.Now()
	for _, pod := range pods.Items {
		if name != "" && pod.GetName() != name {
			continue
		}

		if !hasMatchingAnnotations(pod.GetAnnotations(), matchAnnotations) {
			continue
		}

		expirationTime := pod.GetCreationTimestamp().Time.Add(time.Duration(ttl) * time.Second)
		if now.After(expirationTime) {
			fmt.Printf("Deleting expired pod: %s\n", pod.GetName())
			err := Podclient.Delete(context.TODO(), pod.GetName(), metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("Failed to delete pod %s: %v\n", pod.GetName(), err)
			}
		}
	}
}

func deleteExpiredDeployment(client dynamic.Interface, namespace, name string, ttl int64, matchAnnotations map[string]string) {
	// match appropriate client from dynamic client interface
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "V1",
		Resource: "deployments",
	}

	deployMentClient := client.Resource(gvr).Namespace(namespace)

	deploymentList, err := deployMentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to list deployments: %v\n", err)
		return
	}

	now := time.Now()

	for _, item := range deploymentList.Items {
		if name != "" && item.GetName() != name {
			continue
		}

		if !hasMatchingAnnotations(item.GetAnnotations(), matchAnnotations) {
			continue
		}

		creationTime := item.GetCreationTimestamp().Time

		expirationTime := creationTime.Add(time.Duration(ttl) * time.Second)

		if now.After(expirationTime) {
			fmt.Println("deleting expired deployment:", item.GetName())
			err := client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), item.GetName(), metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("Error deleting the deployment %s , err: %v\n", item.GetName(), err)
			}
		}
	}
}

func hasMatchingAnnotations(resourceAnnotations, requiredAnnotations map[string]string) bool {
	if len(requiredAnnotations) == 0 {
		return true
	}
	for key, value := range requiredAnnotations {
		if resourceAnnotations[key] != value {
			return false
		}
	}
	return true
}
