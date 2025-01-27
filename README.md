# <img src="https://raw.githubusercontent.com/PavelAgarkov/service-template/master/logo.jpg" width="500" height="500"> service-template

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
```

```bash
docker exec -it elasticsearch \
  /usr/share/elasticsearch/bin/elasticsearch-setup-passwords interactive
```
По очереди задаём пароли для elastic, kibana_system, logstash_system и т.д.
Обратите внимание, здесь пароль для elastic должен совпадать с ELASTIC_PASSWORD=elasticpassword из docker-compose.yml.
Для kibana_system введите kibanapassword (соответствует значению в конфиге Kibana).
Kibana подключится к Elasticsearch под логином kibana_system:kibanapassword.

Заходим в Kibana
Открывайте http://localhost:5601.
При появлении формы логина введите elastic / elasticpassword (суперпользователь), или заранее созданный другой пользователь.
Kibana уже будет иметь необходимые права через системного пользователя kibana_system (но внутрь Kibana для UI-работы вы входите логином elastic или другим).