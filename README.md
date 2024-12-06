# service-template

go get -u github.com/swaggo/swag/cmd/swag
go get -u github.com/swaggo/http-swagger
go get -u github.com/alecthomas/template

go install github.com/swaggo/swag/cmd/swag


swag init --generalInfo cmd/simple_http_server/main.go --output ./cmd/simple_http_server/docs



http://localhost:3000/swagger/index.html, http://localhost:3000/swagger/doc.json