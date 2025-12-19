.PHONY: kind
kind:
	kind/create-kind.sh

.PHONY: ingress
ingress:
	kubectl create -f nginx-ingress/nginx-ingress.yaml

.PHONY: cert-manager
cert-manager:
	cert-manager/deploy.sh
	kubectl apply -f cluster-issuer.yaml

.PHONY: charts-repo
charts-repo:
	kubectl create ns charts
	kubectl apply -f charts-repo/dep.yaml

