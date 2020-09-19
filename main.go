package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
)

type Link struct {
	PK  string
	SK  string
	Url string
}

type Visit struct {
	PK       string
	SK       string
	IP       string
	Language string
	Referer  string
	Agent    string
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	const ROUTE_ID string = "routeId"
	const IP string = "x-forwarded-for"
	const LANGUAGE string = "accept-language"
	const REFERER string = "referer"
	const AGENT string = "user-agent"

	//get the url we need to redirect to
	var redirectUrl Link
	routeId := req.PathParameters[ROUTE_ID]

	//link visit is the information about the person visiting the link
	Visit := &Visit{
		PK:       fmt.Sprintf("LINK#%s", routeId),
		SK:       fmt.Sprintf("VISIT#%s", fmt.Sprint(time.Now().UnixNano()/1e6)),
		IP:       req.Headers[IP],
		Language: req.Headers[LANGUAGE],
		Referer:  req.Headers[REFERER],
		Agent:    req.Headers[AGENT],
	}

	db := dynamo.New(session.New(), &aws.Config{Region: aws.String("us-east-2")})
	table := db.Table("linker-v1")

	lookupErr := table.Get("PK", fmt.Sprintf("LINK#%s", routeId)).Range("SK", dynamo.Equal, "USER#dan@strop.com").One(&redirectUrl)

	if lookupErr != nil {
		fmt.Println("LOOKUP ERR " + lookupErr.Error())
	}

	err := table.Put(Visit).Run()

	if err != nil {
		fmt.Println("error:" + err.Error())
	}

	fmt.Print("RedirectURL")
	fmt.Println(redirectUrl)

	fmt.Println("URL:" + redirectUrl.Url)

	proxyRes := events.APIGatewayProxyResponse{
		Headers:    map[string]string{"location": redirectUrl.Url},
		StatusCode: 303,
	}

	return proxyRes, nil

}

/*
$env:GOOS = "linux"
go build -o main main.go
C:\Users\Daniewl\Documents\GitHub\go-workspace\bin\build-lambda-zip.exe -output main.zip main
*/

func main() {
	lambda.Start(Handler)
}
