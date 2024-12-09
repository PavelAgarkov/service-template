#!/bin/bash

function info() {
  echo -e "\e[30;47m build_protoc\e[0m - сборка protoc контейнера"
  echo -e "\e[30;47m rebuild_pb\e[0m - пересборка pb"
  echo -e "\e[30;47m remove_old_pb\e[0m - удаление старых pb"
  echo -e "\e[30;47m run_tests\e[0m - запуск тестов"
  echo -e "\e[30;47m start_containers\e[0m - запуск контейнеров"
  echo -e "\e[30;47m stop_containers\e[0m - остановка контейнеров"
  echo -e "\e[30;47m create_goose_migration\e[0m - создание миграции goose"
  echo -e "\e[30;47m run_goose_migration\e[0m - запуск миграций goose"
  echo -e "\e[30;47m run_goose_migration_down\e[0m - откат миграций goose"
  echo -e "\e[30;47m init_project\e[0m - инициализация проекта"
  echo -e "\e[30;47m build_swagger_docs\e[0m - сборка swagger документации"
}

function run_tests() {
    go test ./...
}

function start_containers() {
    docker compose -f docker-compose-prode.yaml up
}

function stop_containers() {
    docker compose -f docker-compose-prode.yaml down
}

function create_goose_migration() {
    if [ -z "$1" ]; then
        echo "Error: You must provide a migration name."
        echo "Usage: create_goose_migration <migration_name>"
        exit 1
    fi
    goose -dir ./migrations create "$1" sql
}

function run_goose_migration() {
    goose -dir migrations postgres "host=0.0.0.0 user=habrpguser password=pgpwd4habr dbname=habrdb sslmode=disable" up
}

function run_goose_migration_down() {
    goose -dir migrations postgres "host=0.0.0.0 user=habrpguser password=pgpwd4habr dbname=habrdb sslmode=disable" down
}

function init_project() {
    build_protoc  && \
    echo -e "\e[30;43m protoc builded\e[0m" && \
  rebuild_pb && \
  echo -e "\e[30;43m pb rebuilded\e[0m" && \
  build_swagger_docs && \
  echo -e "\e[30;43m swagger docs builded\e[0m" && \
  go install github.com/pressly/goose/v3/cmd/goose@latest && \
  echo -e "\e[30;43m goose installed\e[0m" && \
  run_goose_migration && \
  echo -e "\e[30;43m goose migrations runned\e[0m"
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
    echo "Usage: $0 <function_name> [args...]"
    echo "Available functions: info, start_containers, stop_containers, create_goose_migration, etc."
    exit 1
fi

# Вызываем функцию по имени, если она существует
if declare -f "$1" > /dev/null; then
    # Передаем все последующие аргументы в функцию
    "$@"
else
    echo "Error: Function '$1' not found."
    echo "Available functions: info, start_containers, stop_containers, create_goose_migration, etc."
    exit 1
fi

