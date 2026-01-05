# Installations steps

### [Production Deployment](#production-deployment) 

```bash
helm repo add wafie https://charts.wafie.io
helm repo update 
helm install wafie wafie/wafie \
  --set api.ingress.host="wafie-api.stg.wafie.io" \
  --set api.ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod \
  --set console.ingress.host="wafie-console.stg.wafie.io" \
  --set console.ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod
```

### [Local Dev Deployment](#local-dev-deployment)

### Production deployment




#### Local Dev Deployment
```bash
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
```

kubectl create -f [nginx-ingress.yaml](ops/kind/nginx-ingress.yaml)
### Deploy Nginx Ingress Controller from [nginx-ingress.yaml](ops/kind/nginx-ingress.yaml)
```bash
kubectl create -f ops/kind/nginx-ingress.yaml
```

For being able to access the services from valid URL, export your IP address

Deploy sample application. 
Note the `ingress.hostname` show include your local IP address.
```bash 
helm install wp oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set image.repository=bitnamilegacy/wordpress \
  --set mariadb.image.repository=bitnamilegacy/mariadb \
  --set global.security.allowInsecureImages=true \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.$(ipconfig getifaddr en0).nip.io \
  --set service.type=ClusterIP
```

Once deployed, try access `http://wp.$(ipconfig getifaddr en0).nip.io`
You should get WordPress website. 

Deploy wafie helm chart 
```bash
helm repo add wafie https://charts.wafie.io
helm repo update 
helm upgrade -i wafie wafie/wafie \
  --set api.ingress.host=wafie-api.$(ipconfig getifaddr en0).nip.io \
  --set console.ingress.host=wafie-console.$(ipconfig getifaddr en0).nip.io \
  --set api.ingress.tls=false \
  --set console.ingress.tls=false 
```

Check all wafie pods are running 
```bash
kubectl get pods -l 'app in (wafie-relay,appsecgw,wafie-control-plane)'
```

