#!/bin/sh
set -e
export ETCDCTL_API=3

# Запускаем etcd в фоновом режиме
etcd &

# Сохраняем PID процесса etcd
ETCD_PID=$!

# Ожидание готовности etcd
while ! etcdctl --endpoints=http://localhost:2379 endpoint health > /dev/null 2>&1; do
  echo "Waiting for etcd to be ready..."
  sleep 1
done

echo "etcd is up and running."

# Создаём root пользователя и роль
etcdctl --endpoints=http://localhost:2379 user add root:rootpassword || echo "Root user already exists."
etcdctl --endpoints=http://localhost:2379 role add root || echo "Root role already exists."
etcdctl --endpoints=http://localhost:2379 role grant-permission root readwrite --prefix /
etcdctl --endpoints=http://localhost:2379 user grant-role root root

# Создаём admin пользователя и роль
etcdctl --endpoints=http://localhost:2379 user add admin:adminpassword || echo "Admin user already exists."
etcdctl --endpoints=http://localhost:2379 role add admin || echo "Admin role already exists."
etcdctl --endpoints=http://localhost:2379 role grant-permission admin readwrite --prefix /
etcdctl --endpoints=http://localhost:2379 user grant-role admin admin

# Добавляем задержку перед включением аутентификации
sleep 2

# Включаем аутентификацию
etcdctl --endpoints=http://localhost:2379 auth enable || echo "Authentication is already enabled."

# Ожидаем завершения процесса etcd (чтобы контейнер не завершился)
wait $ETCD_PID
