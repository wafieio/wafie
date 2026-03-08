helm install wp oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set image.repository=bitnamilegacy/wordpress \
  --set mariadb.image.repository=bitnamilegacy/mariadb \
  --set global.security.allowInsecureImages=true \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.stg.wafie.io \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod \
  --set service.type=ClusterIP \
  --kubeconfig ~/.kube/wafie-stg

helm install wp oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set image.repository=bitnamilegacy/wordpress \
  --set mariadb.image.repository=bitnamilegacy/mariadb \
  --set global.security.allowInsecureImages=true \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.$(ipconfig getifaddr en0).nip.io \
  --set service.type=ClusterIP

.staging.wafie.io

helm install cwaf-pg oci://registry-1.docker.io/bitnamicharts/postgresql \
  --set auth.postgresPassword=cwafpg \
  --set auth.username=cwafpg \
  --set auth.password=cwafpg \
  --set auth.database=cwaf


helm install wp oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.apps.user-rhos-01-01.servicemesh.rhqeaws.com \
  --set service.type=ClusterIP \
  --set volumePermissions.enabled=true \
  --set mariadb.volumePermissions.enabled=true


helm template wafie wafie/wafie \
  --set api.host=api.staging.wafie.io \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod



helm template oci://registry-1.docker.io/bitnamicharts/wordpress \
  --set image.repository=bitnamilegacy/wordpress \
  --set mariadb.image.repository=bitnamilegacy/mariadb \
  --set global.security.allowInsecureImages=true \
  --set ingress.enabled=true \
  --set ingress.hostname=wp.staging.wafie.io \
  --set ingress.annotations."cert-manager\.io/cluster-issuer"=letsencrypt-prod \
  --set service.type=ClusterIP | grep image


helm repo add runix https://helm.runix.net

helm install pgadmin4 runix/pgadmin4 \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=pgadmin.10.100.102.95.nip.io \
  --set ingress.hosts[0].paths[0].path="/" \
  --set ingress.hosts[0].paths[0].pathType="Prefix" \
  --set ingress.ingressClassName=nginx \
  --set persistentVolume.enabled=true


helm repo add openebs https://openebs.github.io/openebs
helm repo update
helm install openebs openebs/openebs \
  --set engines.replicated.mayastor.enabled=false \
  --set engines.local.lvm.enabled=false \
  --set engines.local.zfs.enabled=false \
  --set loki.enabled=false \
  --set alloy.enabled=false \
  --namespace openebs \
  --create-namespace


helm repo add openebs-nfs https://openebs-archive.github.io/dynamic-nfs-provisioner
helm repo update
helm install openebs-nfs openebs-nfs/nfs-provisioner \
  --namespace openebs \
  --create-namespace \
  --set nfsStorageClass.backendStorageClass="openebs-nfs"


postgresql:
  global:
    security:
      allowInsecureImages: true
  image:
    repository: bitnamilegacy/postgresql

helm upgrade -i bowapp bow/bagofwords \
  --set host=bow.192.168.64.8.nip.io \
  --set ingress.tls=false \
  --set postgresql.enabled=false


helm upgrade -i bowapp bow/bagofwords \
  --set host=bow.192.168.64.8.nip.io \
  --set ingress.tls.enabled=false \
  --set postgresql.auth.username=bowapp \
  --set postgresql.auth.password=bowapp \
  --set postgresql.auth.database=bowapp \
  --set postgresql.global.security.allowInsecureImages=true \
  --set postgresql.image.repository=bitnamilegacy/postgresql \
  --set postgresql.primary.livenessProbe.enabled=false \
  --set postgresql.primary.readinessProbe.enabled=false \
  --set postgresql.primary.startupProbe.enabled=false \
  --set postgresql.livenessProbe.enabled=false \
  --set postgresql.readinessProbe.enabled=false \
  --set postgresql.startupProbe.enabled=false \
  --set config.smtp.port=25 \
  --set config.smtp.host="foo-bar" \
  --set config.smtp.host="foo-bar" \
  --set config.smtp.username="foo-bar" \
  --set config.smtp.password="foo-bar" \
  --set config.smtp.from_email="foo@bar.com" \
  --set config.smtp.from_name="Foo Bar"
