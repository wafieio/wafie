#!/bin/bash
set +e  # Continue on errors

COLOR_BLUE="\033[0;94m"
COLOR_GREEN="\033[0;92m"
COLOR_RESET="\033[0m"
go install github.com/go-delve/delve/cmd/dlv@latest
# Print useful output for user
echo -e "${COLOR_BLUE}
     %########%      
     %###########%       ____                 _____                      
         %#########%    |  _ \   ___ __   __ / ___/  ____    ____   ____ ___ 
         %#########%    | | | | / _ \\\\\ \ / / \___ \ |  _ \  / _  | / __// _ \\
     %#############%    | |_| |(  __/ \ V /  ____) )| |_) )( (_| |( (__(  __/
     %#############%    |____/  \___|  \_/   \____/ |  __/  \__,_| \___\\\\\___|
 %###############%                                  |_|
 %###########%${COLOR_RESET}


Welcome to your development container!

This is how you can work with it:
- Files will be synchronized between your local machine and this container
- Some ports will be forwarded, so you can access this container via localhost
- Run \`${COLOR_GREEN}discovery-agent${COLOR_RESET}\`
- Run \`${COLOR_GREEN}run-envoy${COLOR_RESET}\`
- Run \`${COLOR_GREEN}build-filter${COLOR_RESET}\`
- Run \`${COLOR_GREEN}pcp${COLOR_RESET}\`
- Run \`${COLOR_GREEN}gateway${COLOR_RESET}\`
- Run \`${COLOR_GREEN}xproc${COLOR_RESET}\`

"

# Set terminal prompt
export PS1="\[${COLOR_BLUE}\]devspace\[${COLOR_RESET}\] ./\W \[${COLOR_BLUE}\]\\$\[${COLOR_RESET}\] "
if [ -z "$BASH" ]; then export PS1="$ "; fi
export SRC_ROOT="/go/src/github.com/wafieio/wafie"
touch /tmp/a

echo "alias discovery-agent=\"cd ${SRC_ROOT} && dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient cmd/agent/discovery/main.go -- start\"" >> /tmp/a
echo "alias gateway=\"cd ${SRC_ROOT} && dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient gateway/cmd/main.go -- start --api-addr=http://wafie-control-plane \"" >> /tmp/a
echo "alias server=\"cd ${SRC_ROOT} && dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient cmd/apiserver/main.go -- start --db-host=wafy-postgresql \"" >> /tmp/a
echo "alias xproc=\"cd ${SRC_ROOT} && dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient xproc/cmd/main.go -- start --api-addr=http://wafie-control-plane \"" >> /tmp/a
echo "alias run-envoy=\"envoy -c ops/envoy/envoy.yaml\"" >> /tmp/a
echo "alias build-filter=\"go build -ldflags='-s -w' -o ./kubeguard-modsec.so -buildmode=c-shared ./cmd/modsecfilter\"" >> /tmp/a
# Include project's bin/ folder in PATH
export PATH="./bin:$PATH"

# Open shell
bash --init-file /tmp/a