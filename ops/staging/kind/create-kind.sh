cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: kind
networking:
  apiServerAddress: 63.250.56.44
  apiServerPort: 6443
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30980
    hostPort: 80
    protocol: TCP
  - containerPort: 30943
    hostPort: 443
    protocol: TCP
EOF
