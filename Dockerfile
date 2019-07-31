FROM golang:1.12-alpine

RUN apk add musl-dev git bash curl ncurses

WORKDIR /tmp
RUN curl 'https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-install-linux-4.1.8.tar.gz' > openshift-install.tar.gz
RUN tar -xzf openshift-install.tar.gz
RUN mv openshift-install /usr/bin/
RUN rm openshift-install.tar.gz

RUN adduser -D autocluster # -D = no password

RUN mkdir -p /etc/auto-cluster
RUN chgrp -R autocluster /etc/auto-cluster

USER autocluster
WORKDIR /home/autocluster

COPY --chown=autocluster:autocluster go.mod go.sum /home/autocluster/

RUN go mod download

COPY --chown=autocluster:autocluster main.go .
RUN go build -o auto-cluster .

COPY --chown=autocluster:autocluster scripts scripts

CMD /home/autocluster/auto-cluster