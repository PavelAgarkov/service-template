#!/bin/bash

function info() {
  echo "unit_project - первый запуск проекта"
  echo "build_protoc - сборка protoc контейнера"
  echo "rebuild_pb - пересборка pb для спецификаций"
  echo "remove_old_pb - remove old pb"
  echo "build_swagger_docs - собрать swagger спецификации"
}

function unit_project() {
    build_protoc  && \
  rebuild_pb && \
  build_swagger_docs
}

function build_swagger_docs() {
  go get -u github.com/swaggo/swag/cmd/swag && \
  go get -u github.com/swaggo/http-swagger && \
  go get -u github.com/alecthomas/template && \
  go install github.com/swaggo/swag/cmd/swag && \
    swag init --generalInfo cmd/simple_http_server/main.go --output ./cmd/simple_http_server/docs
}

# Определяем функции
function build_protoc() {
    docker build -t protoc:25.1 -f protoc.dockerfile .
}

function rebuild_pb() {
  docker run -v $(pwd):/cmd protoc:25.1 protoc --go_out=./cmd/grps_server/pb --go-grpc_out=./cmd/grps_server/pb ./cmd/grps_server/proto/myservice.proto && \
    docker run -v $(pwd):/cmd protoc:25.1 protoc --go_out=./cmd/grps_server/pb --go-grpc_out=./cmd/grps_server/pb ./cmd/grps_server/proto/myservice2.proto
}

function remove_old_pb() {
    sudo rm -rf cmd/grps_server/pb/myservice2 && \
        sudo rm -rf cmd/grps_server/pb/myservice
}

# Проверяем, передан ли аргумент
if [ -z "$1" ]; then
    echo "Usage: $0 <function_name>"
    echo "Available functions: hello, goodbye, current_time"
    exit 1
fi

# Вызываем функцию по имени, если она существует
if declare -f "$1" > /dev/null; then
    "$1" # Вызов функции по имени
else
    echo "Error: Function '$1' not found."
    echo "Available functions: hello, goodbye, current_time"
    exit 1
fi
