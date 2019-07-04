FROM cfcontainerization/cf-operator-base
COPY binaries/cf-operator-linux-amd64 /usr/local/bin/cf-operator
ENTRYPOINT ["/usr/local/bin/cf-operator"]
