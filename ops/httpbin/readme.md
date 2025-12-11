### Install `httpbin`

helm repo add wafie https://charts.wafie.io
helm repo update
helm install httpbin wafie/httpbin \
--set baseHost=$(ipconfig getifaddr en0).nip.io