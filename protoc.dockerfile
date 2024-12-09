# Используем официальный образ Ubuntu в качестве базового
FROM ubuntu:20.04

# Устанавливаем временную зону, чтобы избежать интерактивного запроса
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    ln -sf /usr/share/zoneinfo/Asia/Bishkek /etc/localtime && \
    echo "Asia/Bishkek" > /etc/timezone && \
    apt-get install -y tzdata && \
    dpkg-reconfigure -f noninteractive tzdata

# Устанавливаем обновления и необходимые инструменты
RUN apt-get update && \
    apt-get install -y \
        curl \
        unzip \
        git \
        build-essential \
        golang && \
    rm -rf /var/lib/apt/lists/*

# Устанавливаем protoc
ENV PROTOC_VERSION=25.1
RUN curl -L -o /tmp/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \
    unzip /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

# Устанавливаем плагины protoc-gen-go и protoc-gen-go-grpc
ENV GO111MODULE=on
ENV PATH="/root/go/bin:${PATH}"
RUN go mod init example.com/temp && \
    go get google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0 && \
    go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0 && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

# Проверяем установку
RUN protoc --version && \
    protoc-gen-go --version && \
    protoc-gen-go-grpc --version

# Создаем рабочую директорию
WORKDIR /cmd

# Открываем возможность монтирования через WORKDIR
VOLUME /cmd

# Устанавливаем CMD для запуска контейнера с bash
CMD ["bash"]