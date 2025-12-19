kind:
	kind/create-kind.sh

ingress:
	kubectl create -f nginx-ingress/nginx-ingress.yaml

cert-manager:
	cert-manager/deploy.sh
	kubectl apply -f cluster-issuer.yaml

charts-repo:
	kubectl create ns charts
	kubectl apply -f charts-repo/dep.yaml

