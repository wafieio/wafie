# Installations steps

### Create local Kind cluster 
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
helm install wafie wafie/wafie \
  --set baseHost=$(ipconfig getifaddr en0).nip.io
```

Check all wafie pods are running 
```bash
kubectl get pods -l 'app in (wafie-relay,appsecgw,wafie-control-plane)'
```

Optional - Deploy OpenSearch Dashboard 
```bash
helm install od wafie/opensearch-dashboards \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=opensearch-dashbard.$(ipconfig getifaddr en0).nip.io \
  --set ingress.hosts[0].paths[0].path="/" \
  --set ingress.hosts[0].paths[0].backend.service.name="opensearch-dashboards" \
  --set ingress.hosts[0].paths[0].backend.service.port.name="http"
```

List all discovered applications
```bash
curl --location 'http://wafie-api.192.168.1.51.nip.io/wafie.v1.ApplicationService/ListApplications' \
--header 'Content-Type: application/json' \
--data '{
    "options": {
        "include_ingress": false
    }
}'
```
Enable protection for selected application 
```bash
curl --location 'http://wafie-api.192.168.1.51.nip.io/wafie.v1.ProtectionService/CreateProtection' \
--header 'Content-Type: application/json' \
--data '{
    "application_id": 1,
    "desired_state": {
        "mode_sec": {
            "paranoia_level": "PARANOIA_LEVEL_4",
            "protection_mode": "PROTECTION_MODE_ON"
        }
    },
    "protection_mode": "PROTECTION_MODE_ON"
}'
```
