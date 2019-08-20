FROM alpine:latest

RUN apk add --update musl-dev git curl go

RUN adduser -D app
USER app
WORKDIR /home/app

COPY --chown=app:app go.mod go.sum main.go /home/app
COPY --chown=app:app cluster/ cluster/
COPY --chown=app:app config/ config/

RUN go build
CMD /home/app/auto-cluster
