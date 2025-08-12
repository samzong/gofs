# gofs - Simple HTTP File Server

A lightweight, fast HTTP file server written in Go that serves files with directory browsing, multiple theme support, WebDAV, and advanced features like concurrent ZIP downloads.

## Quick Start

### Pull and Run

```bash
# Pull from Docker Hub
docker pull samzong/gofs:latest

# Run with current directory
docker run -p 8000:8000 -v $(pwd):/data:ro samzong/gofs:latest

# Run with custom directory
docker run -p 8000:8000 -v /path/to/files:/data:ro samzong/gofs:latest
```

### Docker Compose

```yaml
version: '3'
services:
  gofs:
    image: samzong/gofs:latest
    ports:
      - "8000:8000"
    volumes:
      - /path/to/files:/data:ro
    environment:
      - GOFS_HOST=0.0.0.0
      - GOFS_PORT=8000
      - GOFS_THEME=advanced
```

## Kubernetes Deployment

### Basic Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gofs
  labels:
    app: gofs
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gofs
  template:
    metadata:
      labels:
        app: gofs
    spec:
      containers:
      - name: gofs
        image: samzong/gofs:latest
        ports:
        - containerPort: 8000
        env:
        - name: GOFS_HOST
          value: "0.0.0.0"
        - name: GOFS_PORT
          value: "8000"
        - name: GOFS_THEME
          value: "advanced"
        volumeMounts:
        - name: data
          mountPath: /data
          readOnly: true
        resources:
          requests:
            memory: "32Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8000
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8000
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: data
        hostPath:
          path: /path/to/files
          type: Directory
---
apiVersion: v1
kind: Service
metadata:
  name: gofs
spec:
  selector:
    app: gofs
  ports:
  - port: 80
    targetPort: 8000
  type: ClusterIP
```

### Using ConfigMap for Multi-Directory

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gofs-config
data:
  GOFS_DIR: "/config:/etc:ro:Configuration;/logs:/var/log::App Logs;/data:/app/data::Application Data"
  GOFS_THEME: "advanced"
  GOFS_SHOW_HIDDEN: "false"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gofs-multi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gofs-multi
  template:
    metadata:
      labels:
        app: gofs-multi
    spec:
      containers:
      - name: gofs
        image: samzong/gofs:latest
        ports:
        - containerPort: 8000
        envFrom:
        - configMapRef:
            name: gofs-config
        volumeMounts:
        - name: config-volume
          mountPath: /etc
          readOnly: true
        - name: logs-volume
          mountPath: /var/log
          readOnly: true
        - name: data-volume
          mountPath: /app/data
          readOnly: true
      volumes:
      - name: config-volume
        configMap:
          name: app-config
      - name: logs-volume
        persistentVolumeClaim:
          claimName: logs-pvc
      - name: data-volume
        persistentVolumeClaim:
          claimName: data-pvc
```

### With Ingress (Nginx)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gofs-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/client-max-body-size: "100m"
spec:
  tls:
  - hosts:
    - files.example.com
    secretName: gofs-tls
  rules:
  - host: files.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gofs
            port:
              number: 80
```

### StatefulSet with Persistent Storage

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: gofs-stateful
spec:
  serviceName: gofs-headless
  replicas: 1
  selector:
    matchLabels:
      app: gofs-stateful
  template:
    metadata:
      labels:
        app: gofs-stateful
    spec:
      containers:
      - name: gofs
        image: samzong/gofs:latest
        ports:
        - containerPort: 8000
        env:
        - name: GOFS_THEME
          value: "advanced"
        - name: GOFS_ENABLE_WEBDAV
          value: "true"
        volumeMounts:
        - name: data-storage
          mountPath: /data
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
  volumeClaimTemplates:
  - metadata:
      name: data-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: gofs-headless
spec:
  clusterIP: None
  selector:
    app: gofs-stateful
  ports:
  - port: 8000
```

### Helm Chart Values Example

```yaml
# values.yaml
image:
  repository: samzong/gofs
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 80
  targetPort: 8000

ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/client-max-body-size: "100m"
  hosts:
    - host: files.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: gofs-tls
      hosts:
        - files.example.com

config:
  theme: advanced
  showHidden: false
  enableWebdav: true
  auth: ""  # username:password format

persistence:
  enabled: true
  storageClass: ""
  accessMode: ReadWriteOnce
  size: 10Gi
  mountPath: /data

resources:
  requests:
    memory: 64Mi
    cpu: 100m
  limits:
    memory: 256Mi
    cpu: 500m

nodeSelector: {}
tolerations: []
affinity: {}
```

## Configuration

### Environment Variables

- `GOFS_HOST` - Host to bind (default: 0.0.0.0)
- `GOFS_PORT` - Port to bind (default: 8000)
- `GOFS_THEME` - Theme to use (default, classic, advanced)
- `GOFS_SHOW_HIDDEN` - Show hidden files (true/false)
- `GOFS_AUTH` - Enable authentication (username:password)
- `GOFS_ENABLE_WEBDAV` - Enable WebDAV server (true/false)

### Multi-Directory Mounting

```bash
docker run -p 8000:8000 \
  -v /etc:/config:ro \
  -v /var/log:/logs:ro \
  -e GOFS_DIR="/config:/etc:ro:Configuration;/logs:/var/log::App Logs" \
  samzong/gofs:latest
```

## Features

- 🚀 **Fast & Lightweight** - Single binary, minimal resource usage
- 📁 **Directory Browsing** - Clean, responsive directory listings
- 🎨 **Multiple Themes** - Default, classic, and advanced themes
- 📱 **Mobile Friendly** - Responsive design works on all devices
- 🔐 **Authentication** - Optional HTTP basic authentication
- 🌐 **WebDAV Support** - Read-only WebDAV server for file access
- 📦 **ZIP Downloads** - Bulk download folders as ZIP files
- 🔄 **Multi-Directory** - Serve multiple directories with custom names
- ⚡ **HTTP/2 Support** - Modern HTTP/2 protocol support
- 🛡️ **Security** - Path traversal protection and secure headers

## Supported Tags

- `latest` - Latest stable release
- `v0.2.x` - Specific version tags
- Multi-architecture support: `linux/amd64`, `linux/arm64`

## Source Code

- GitHub: [https://github.com/samzong/gofs](https://github.com/samzong/gofs)
- Issues: [https://github.com/samzong/gofs/issues](https://github.com/samzong/gofs/issues)

## License

MIT License - see [LICENSE](https://github.com/samzong/gofs/blob/main/LICENSE) for details.