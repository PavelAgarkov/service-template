# service-template

Service template for golang microservices with grpc, grpc-gateway, swagger, goose, postgres, redis, docker-compose.
It is resolving the following tasks:
- [x] Http\s server\client, sse server\client
- [x] Websocket server\client
- [x] gRPC\s server\client
- [x] gRPC server
- [x] gRPC gateway
- [x] Swagger
- [x] Postgres
- [x] RabbitMQ
- [x] Redis
- [x] Distributed Cron
- [x] Goose
- [x] Docker-compose
- [x] helper.sh
- [x] docker file for protoc

It helps to start new project with all necessary components. And it have graceful shutdown for all components and a smooth mechanism for shutdown.
There have docker-compose for local development. 
Template have a lot of examples for different types of communication between services.
Template have smooth mechanism for refactorings and adding new features.
Template have custom DI container for services. 

You can just change code if you need to add new features or refactor existing code.
You can see the example of usage in the `cmd` folder.
You can see tasks for todo in issues.

## Command list in helper.sh
```bash
build_protoc - сборка protoc контейнера
rebuild_pb - пересборка pb
remove_old_pb - удаление старых pb
run_tests - запуск тестов
start_containers - запуск контейнеров
stop_containers - остановка контейнеров
create_goose_migration - создание миграции goose
run_goose_migration - запуск миграций goose
run_goose_migration_down - откат миграций goose
init_project - инициализация проекта
build_swagger_docs - сборка swagger документации