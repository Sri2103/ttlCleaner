# ResourceTTL Controller

A lightweight Kubernetes controller built using `client-go` that enables TTL (time-to-live) based cleanup of Kubernetes resources using a custom resource definition (CRD) named `ResourceTTL`.

This controller watches for `ResourceTTL` custom resources and deletes targeted Kubernetes resources (like Pods, Deployments, etc.) after a specified time duration based on their creation timestamp and optional annotation filters.

---

## ✨ Features

- Watches for custom `ResourceTTL` resources
- Supports TTL-based cleanup of:
  - Pods (via typed client)
  - Deployments (via dynamic client)
- Filters resources by name and annotations
- Supports namespaced resources
- Built using raw `client-go` (no Kubebuilder)

---

## 🧱 CRD Example

Here's a sample `ResourceTTL` custom resource:

```yaml
apiVersion: cleanup.example.com/v1
kind: ResourceTTL
metadata:
  name: cleanup-old-deployments
spec:
  resourceKind: Deployment
  resourceGroup: apps
  resourceVersion: v1
  resourceNamePlural: deployments
  namespace: default
  ttlSeconds: 3600
  matchAnnotations:
    environment: dev
```

`kubectl apply -f manifests/crd.yaml`

`go run main.go`

`kubectl apply -f manifests/example.yaml`

## 🔧 How It Works

Uses a dynamic informer to watch ResourceTTL custom resources.

On addition, the controller reads the spec, constructs the appropriate GroupVersionResource, and uses the dynamic client to list and delete expired resources.

TTL is calculated as:
resource.CreationTimestamp + ttlSeconds

## 📌 Supported Resource Kinds

✅ Pod (via typed client)

✅ Deployment (via dynamic client)

🚧 Other resource kinds can be easily added by extending the logic.

## 📚 Requirements

Kubernetes cluster (v1.20+ recommended)

Go 1.20+

client-go library
