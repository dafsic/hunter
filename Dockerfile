ARG REGISTRY

FROM ${REGISTRY}alpine:3.15.4

WORKDIR /app

COPY ./bin/hunter hunter
COPY ./static static

#CMD ["apiserver","-c","config.toml"]

#ENTRYPOINT ["/app/pm"]