# Kubernetes Deployment Examples

This directory contains Kubernetes deployment examples for running gofs as a sidecar container to serve files alongside your main application.

## Overview

The sidecar pattern allows gofs to serve files from a shared volume:
- **Main App**: Writes files to the shared volume (e.g., `/app/public`)
- **gofs Sidecar**: Serves files from the shared volume (e.g., `/data`)

## Deployment

Deploy with a Deployment, Service, and Ingress:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-with-gofs-sidecar
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-application
  template:
    metadata:
      labels:
        app: my-application
    spec:
      volumes:
      - name: shared-files
        emptyDir: {}
        # Use persistentVolumeClaim for persistent storage:
        # persistentVolumeClaim:
        #   claimName: shared-files-pvc

      containers:
      # Main application container
      - name: main-app
        image: my-application:1.0
        ports:
        - containerPort: 3000
        volumeMounts:
        - name: shared-files
          mountPath: /app/public
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"

      # gofs sidecar container
      - name: gofs-sidecar
        image: your-registry/gofs:latest
        args: 
        - "--host=0.0.0.0"
        - "--port=8000"
        - "--dir=/data"
        ports:
        - containerPort: 8000
        volumeMounts:
        - name: shared-files
          mountPath: /data
        livenessProbe:
          exec:
            command: ["/gofs", "--health-check"]
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          exec:
            command: ["/gofs", "--health-check"]
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        securityContext:
          runAsNonRoot: true
          runAsUser: 65534
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
          readOnlyRootFilesystem: true

---
apiVersion: v1
kind: Service
metadata:
  name: app-service
spec:
  selector:
    app: my-application
  ports:
  - name: app-http
    port: 80
    targetPort: 3000
  - name: files-http
    port: 8000
    targetPort: 8000
  type: ClusterIP

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: myapp.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-service
            port:
              number: 80
      - path: /files
        pathType: Prefix
        backend:
          service:
            name: app-service
            port:
              number: 8000
```

## Configuration

### Arguments

```yaml
args:
  - "--host=0.0.0.0"      # Listen on all interfaces
  - "--port=8000"         # Port to serve on
  - "--dir=/data"         # Directory to serve
  - "--theme=advanced"     # UI theme (optional)
  - "--show-hidden"       # Show hidden files (optional)
```

### Volume Types

- **emptyDir**: Temporary storage, deleted when Pod stops
- **persistentVolumeClaim**: Persistent storage across Pod restarts

## Use Cases

- **Static Assets**: Main app generates files, gofs serves them
- **Reports**: Main app creates reports, gofs provides download access
- **File Uploads**: External uploads via gofs, main app processes files

## Quick Start

1. **Build and Push Docker Image**:
   ```bash
   docker build -t your-registry/gofs:latest .
   docker push your-registry/gofs:latest
   ```

2. **Deploy**:
   ```bash
   kubectl apply -f deployment.yaml
   ```

3. **Verify**:
   ```bash
   kubectl get deployments
   kubectl get services
   kubectl get ingress
   ```

## Troubleshooting

### Debug Commands

```bash
# Check container logs
kubectl logs <pod-name> -c gofs-sidecar

# Test health endpoints
kubectl exec -it <pod-name> -c gofs-sidecar -- wget -O- http://localhost:8000/healthz
```