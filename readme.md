# Wafie - AI ready, Kubernetes native WAF and API protection platform

## Why? 
The core mission of Wafie is to accomplish a very different and broad set of application networking (L7) tasks
in a simple, fast, and scalable manner. A wide variety of functions, including basic security controls, 
access management, rate limiting, and request/response transformation, can be performed.

## For who?
If you are a Developer/DevOps/SRE/Platform engineer who runs workloads on Kubernetes cluster
and looking for unified middleware to manage applications L7 networking 
and security without changing the existing code-base, Wafie can help you with that.

## How?
* **Kubernetes-Native**: Designed from the ground up to leverage Kubernetes primitives for configuration, 
integration, and deployment, making it seamless for cloud-native teams
* **AI-Ready**: Simply describe the desired functionality in plain English 
once the Wafie agent is connected to the model, and the Wafie engine will handle the rest.
* **Unified Platform**: Serving as a single control point for security (WAF), 
traffic management (proxies), and modern API gateway functionality (API protection), 
eliminating the need to "jungle" multiple disparate tools.

## Wafie components
* **libmodsecurity** - The core component, [libmodsecurity](https://github.com/owasp-modsecurity/ModSecurity), 
is a firewall engine that processes CRS rules. 
These rules are defined in SecLang, a language easily understood by any AI agent. 
This synergy means that humans can express security or networking requirements in plain English, 
and an AI agent can seamlessly translate them into SecLang for the libmodsecurity engine to execute.
* **envoy** - the cloud native proxy server
* **wafie:ext_proc** - [envoy external processing](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter)
server acting as a glue between libmodsecurity and envoy proxy
* **wafie:discovery** - discovery agent that watch for your K8s Ingress and Service and automatically generating Envoy control plane configurations
* **wafie:relay** - proxying applications traffic without a need for sidecars containers.
* **wafie:api** - api server



# Installation

## Prerequisites
- Kubernetes cluster (1.19+)
- Helm 3.8+
- kubectl configured to access your cluster

## Installing the Helm Chart

### Install Latest Version

```bash
helm install wafie oci://ghcr.io/wafieio/charts/wafie \
  --set api.ingress.host="wafie-api.example.com" \
  --set console.ingress.host="wafie-console.example.com"
```

### Install Specific Version

```bash
helm install wafie oci://ghcr.io/wafieio/charts/wafie --version 0.0.2 \
  --set api.ingress.host="wafie-api.example.com" \
  --set console.ingress.host="wafie-console.example.com"
```

### Install with Custom Values File

```bash
helm install wafie oci://ghcr.io/wafieio/charts/wafie -f custom-values.yaml
```

## List Available Versions

### View on GitHub Packages
https://github.com/wafieio/wafie/pkgs/container/charts%2Fwafie

### View on GitHub Releases
https://github.com/wafieio/wafie/releases

### Using API
```bash
curl -s https://ghcr.io/v2/wafieio/charts/wafie/tags/list | jq -r '.tags[]'
```

## Upgrading the Chart

### Upgrade to Latest Version

```bash
helm upgrade wafie oci://ghcr.io/wafieio/charts/wafie
```

### Upgrade to Specific Version

```bash
helm upgrade wafie oci://ghcr.io/wafieio/charts/wafie --version 0.0.2
```

### Upgrade with New Values

```bash
helm upgrade wafie oci://ghcr.io/wafieio/charts/wafie \
  --set api.ingress.host="new-api.example.com" \
  --reuse-values
```

## Uninstalling the Chart

```bash
helm uninstall wafie
```

To also delete the namespace (if dedicated):
```bash
helm uninstall wafie
kubectl delete namespace wafie
```

## Chart Information

### Show Chart Details

```bash
# Show all chart information
helm show all oci://ghcr.io/wafieio/charts/wafie --version 0.0.2

# Show only values
helm show values oci://ghcr.io/wafieio/charts/wafie --version 0.0.2

# Show only readme
helm show readme oci://ghcr.io/wafieio/charts/wafie --version 0.0.2
```

### Check Deployed Status

```bash
# List all releases
helm list

# Get release status
helm status wafie

# Get release values
helm get values wafie
```

---

## Production Deployment Example

```bash
helm install wafie oci://ghcr.io/wafieio/charts/wafie --version 0.0.2 \
  --create-namespace \
  --namespace wafie \
  --set api.ingress.host="wafie-api.stg.wafie.io" \
  --set api.ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod \
  --set console.ingress.host="wafie-console.stg.wafie.io" \
  --set console.ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod
```

## Local Development Deployment

### Local Dev Deployment Setup
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
helm install wafie oci://ghcr.io/wafieio/charts/wafie \
  --set api.ingress.host=wafie-api.$(ipconfig getifaddr en0).nip.io \
  --set console.ingress.host=wafie-console.$(ipconfig getifaddr en0).nip.io \
  --set api.ingress.tls=false \
  --set console.ingress.tls=false
```

Check all wafie pods are running 
```bash
kubectl get pods -l 'app in (wafie-relay,appsecgw,wafie-control-plane)'
```

