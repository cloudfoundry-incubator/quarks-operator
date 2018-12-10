ARG base_image=opensuse/leap:15.0
FROM ${base_image}

RUN zypper -n in system-user-nobody

USER nobody

ADD _output/bin/cf-operator /usr/local/bin/cf-operator
