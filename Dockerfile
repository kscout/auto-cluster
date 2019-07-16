FROM alpine:latest

RUN apk add musl-dev go git

RUN adduser -D autocluster # -D = no password

RUN mkdir -p /etc/auto-cluster
RUN chgrp -R autocluster /etc/auto-cluster

USER autocluster
WORKDIR /home/autocluster

COPY --chown=autocluster:autocluster go.mod go.sum /home/autocluster/

RUN go mod download

COPY --chown=autocluster:autocluster main.go /home/autocluster/
RUN go build -o auto-cluster .

CMD /home/autocluster/auto-cluster
