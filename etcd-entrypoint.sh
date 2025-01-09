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
etcdctl --endpoints=http://localhost:2379 user add root:rootpassword
etcdctl --endpoints=http://localhost:2379 role add root
etcdctl --endpoints=http://localhost:2379 role grant-permission root readwrite --prefix /
etcdctl --endpoints=http://localhost:2379 user grant-role root root

# Создаём admin пользователя и роль
etcdctl --endpoints=http://localhost:2379 user add admin:adminpassword
etcdctl --endpoints=http://localhost:2379 role add admin
etcdctl --endpoints=http://localhost:2379 role grant-permission admin readwrite --prefix /
etcdctl --endpoints=http://localhost:2379 user grant-role admin admin

# Включаем аутентификацию
etcdctl --endpoints=http://localhost:2379 auth enable

# Ожидаем завершения процесса etcd (чтобы контейнер не завершился)
wait $ETCD_PID
