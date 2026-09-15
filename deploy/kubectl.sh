#!/usr/bin/env sh
# kubectl against the cluster from this machine. The node's API server is not
# open to the internet, so this goes through an SSH tunnel; the first run copies
# the kubeconfig, which is the cluster's admin credential and stays out of the
# repository.
#
#   deploy/kubectl.sh -n deucepoint logs deploy/api --since=1h
#   deploy/kubectl.sh -n deucepoint get pods
set -eu

host=deucepoint
port=16443    # not 6443: Docker Desktop's own cluster may be listening there
cfg="$HOME/.kube/$host.yaml"

if [ ! -f "$cfg" ]; then
	mkdir -p "$HOME/.kube"
	# k3s writes the server as 127.0.0.1:6443 and its certificate covers
	# 127.0.0.1, so only the port changes.
	ssh "$host" cat /etc/rancher/k3s/k3s.yaml | sed "s#127.0.0.1:6443#127.0.0.1:$port#" > "$cfg"
	chmod 600 "$cfg"
fi

# An open tunnel answers 401 here; nothing listening is a connection error.
if ! curl -sk --max-time 2 -o /dev/null "https://127.0.0.1:$port/"; then
	ssh -fN -L "$port:127.0.0.1:6443" "$host"
fi

KUBECONFIG="$cfg" exec kubectl "$@"
