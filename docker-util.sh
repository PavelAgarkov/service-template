#!/bin/bash

# Файл для хранения списка docker-compose файлов
DOCKER_COMPOSE_LIST_FILE=".docker-compose-list"

# Относительная директория для docker-compose файлов
DOCKER_DIR="docker"

# Функция для добавления файлов в список
add_files() {
  for file in "$@"; do
    # Полный путь к файлу
    full_path="$DOCKER_DIR/$file"
    if [ -f "$full_path" ]; then
      echo "-f $full_path" >> "$DOCKER_COMPOSE_LIST_FILE"
      echo "Файл $full_path добавлен."
    else
      echo "Ошибка: файл $full_path не существует."
    fi
  done
}

# Функция для запуска контейнеров
start_containers() {
  if [ ! -f "$DOCKER_COMPOSE_LIST_FILE" ] || [ ! -s "$DOCKER_COMPOSE_LIST_FILE" ]; then
    echo "Нет файлов для запуска. Добавьте файлы с помощью команды add."
    return 1
  fi

  echo "Запуск контейнеров с файлами:"
  cat "$DOCKER_COMPOSE_LIST_FILE"
  docker-compose $(cat "$DOCKER_COMPOSE_LIST_FILE") up -d
}

# Функция для остановки контейнеров
stop_containers() {
  if [ ! -f "$DOCKER_COMPOSE_LIST_FILE" ] || [ ! -s "$DOCKER_COMPOSE_LIST_FILE" ]; then
    echo "Нет файлов для остановки. Добавьте файлы с помощью команды add."
    return 1
  fi

  echo "Остановка контейнеров с файлами:"
  cat "$DOCKER_COMPOSE_LIST_FILE"
  docker-compose $(cat "$DOCKER_COMPOSE_LIST_FILE") down
}

# Функция для очистки списка файлов
clear_files() {
  if [ -f "$DOCKER_COMPOSE_LIST_FILE" ]; then
    > "$DOCKER_COMPOSE_LIST_FILE"
    echo "Список файлов очищен."
  else
    echo "Файл $DOCKER_COMPOSE_LIST_FILE не существует."
  fi
}

# Основной код
case "$1" in
  add)
    shift # Убираем первый аргумент (add)
    add_files "$@"
    ;;
  start)
    start_containers
    ;;
  stop)
    stop_containers
    ;;
  clear)
    clear_files
    ;;
  *)
    echo "Использование: $0 {add <file1> <file2> ... | start | stop | clear}"
    exit 1
    ;;
esac