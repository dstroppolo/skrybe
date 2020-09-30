package main

import (
	"context"

	V "linker/visits"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	proxyRes, _ := V.Handle(req)
	return proxyRes, nil
}

/*
$env:GOOS = "linux"
go build -o main main.go
C:\Users\Daniewl\Documents\GitHub\go-workspace\bin\build-lambda-zip.exe -output main.zip main
*/

func main() {
	lambda.Start(Handler)
	//https://medium.com/dm03514-tech-blog/go-aws-lambda-project-structure-using-golang-98b6c0a5339d
}
