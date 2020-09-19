package main

import (
	"context"
	"fmt"
	"os"
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
	const IP string = "X-Forwarded-For"
	const LANGUAGE string = "accept-language"
	const REFERER string = "referer"
	const AGENT string = "User-Agent"

	//get the url we need to redirect to
	var redirectUrl Link
	routeId := req.PathParameters[ROUTE_ID]

	//link visit is the information about the person visiting the link
	Visit := &Visit{
		PK:       fmt.Sprintf("LINK#%s", routeId),
		SK:       fmt.Sprintf("VISIT#%s", fmt.Sprint(time.Now().UnixNano()/1e6)), //get unix time in ms
		IP:       req.Headers[IP],
		Language: req.Headers[LANGUAGE],
		Referer:  req.Headers[REFERER],
		Agent:    req.Headers[AGENT],
	}

	var region = os.Getenv("AWS_REGION")
	var tableName = os.Getenv("DDB_TABLE_NAME")

	db := dynamo.New(session.New(), &aws.Config{Region: aws.String(region)})
	table := db.Table(tableName)

	lookupErr := table.Get("PK", fmt.Sprintf("LINK#%s", routeId)).Range("SK", dynamo.Equal, "USER#dan@strop.com").One(&redirectUrl)

	if lookupErr != nil {
		fmt.Println("LOOKUP ERR " + lookupErr.Error())
	}

	err := table.Put(Visit).Run()

	if err != nil {
		fmt.Println("error:" + err.Error())
	}

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
