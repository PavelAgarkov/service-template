package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"service-template/application"
	"service-template/pkg"
	"service-template/server"
)

func main() {
	logger := pkg.NewLogger(pkg.LoggerConfig{ServiceName: "http-client", LogPath: "logs/app.log"})

	father, cancel := context.WithCancel(context.Background())
	father = pkg.LoggerWithCtx(father, logger)
	defer cancel()

	app := application.NewApp(father, nil, logger)
	app.Start(cancel)
	defer app.Stop()
	defer app.RegisterRecovers()()

	app.RegisterShutdown("logger", func() {
		err := logger.Sync()
		if err != nil {
			log.Println(fmt.Sprintf("failed to sync logger: %v", err))
		}
	}, 101)

	httpClient := server.NewHttpClientConnection(url.URL{Scheme: "http", Host: "localhost:8081"}, 0)
	httpsClient, _ := server.NewHttpsClientConnection(url.URL{Scheme: "https", Host: "localhost:8080"}, "./server.crt", logger, 0)

	Do(httpClient)
	Do(httpsClient)

	//openssl req \
	//-x509 \
	//-nodes \
	//-newkey rsa:4096 \
	//-keyout server.key \
	//-out server.crt \
	//-days 365 \
	//-subj "/CN=localhost" \
	//-addext "subjectAltName=DNS:localhost"

	app.Run()
}

func Do(httpsClient *server.HttpClientConnection) {
	data := []byte(`{
  "name": "John1",
  "age": 30,
  "email": "a@mail.ru"
}`)

	requestBody := bytes.NewBuffer(data)

	//u := url.URL{Host: httpsClient.GetBaseURL(), Path: "/empty"}
	baseURL := httpsClient.GetBaseURL()
	u := baseURL.ResolveReference(&url.URL{Path: "/empty"})
	resp, err := httpsClient.ClientConnection.Post(u.String(), "application/json", requestBody)
	if err != nil {
		log.Println("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read response body: %v", err)
	}

	fmt.Println("Status:", resp.Status)
	fmt.Println("Body:", string(body))
}
