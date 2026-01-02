cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: kind
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

kubectl create -f ../kind/nginx-ingress.yaml
kubectl wait --for=condition=Progressing \
 deployment/ingress-nginx-controller \
 -ningress-nginx \
 --timeout=300s

kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 \
 $(kubectl get rs \
    -lapp.kubernetes.io/component=controller,app.kubernetes.io/name=ingress-nginx \
    -ningress-nginx -oname) \
 -ningress-nginx \
 --timeout=300s

kubectl wait --for=condition=ready pod \
 -lapp.kubernetes.io/name=ingress-nginx,app.kubernetes.io/component=controller \
 -ningress-nginx \
 --timeout=300s



helm install wp oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set image.repository=bitnamilegacy/wordpress \
  --set mariadb.image.repository=bitnamilegacy/mariadb \
  --set global.security.allowInsecureImages=true \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.$(ipconfig getifaddr en0).nip.io \
  --set service.type=ClusterIP