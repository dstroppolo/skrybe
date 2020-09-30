package visits

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
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
	PK          string
	SK          string
	IP          string
	Language    string
	Referer     string
	AgentHeader string
	User        string
	UserAgent   UserAgent
	Location    Location
}

type UserAgent struct {
	IsBot    bool
	IsMobile bool
	Platform string
	OS       string
	Browser  string
}

type Location struct {
	CountryCode string `json:"country_code2"`
	Country     string `json:"country_name"`
	State       string `json:"state_prov"`
}

func Create(event map[string]events.DynamoDBAttributeValue) (Visit, error) {
	visit := &Visit{
		PK:          event["PK"].String(),
		SK:          event["SK"].String(),
		IP:          event["IP"].String(),
		Language:    event["Language"].String(),
		Referer:     event["Referer"].String(),
		AgentHeader: event["AgentHeader"].String(),
	}

	var err error

	if visit == nil {
		err = errors.New("Can't create the visit object")
	}

	return *visit, err
}

func Connect() dynamo.Table {
	var region = os.Getenv("AWS_REGION")
	var tableName = os.Getenv("DDB_TABLE_NAME")

	db := dynamo.New(session.New(), &aws.Config{Region: aws.String(region)})
	table := db.Table(tableName)
	return table
}

func Handle(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	const ROUTE_ID string = "routeId"
	const IP string = "x-forwarded-for" //might have to check cuz something fucky happens with capital letters for this and user agent
	const LANGUAGE string = "accept-language"
	const REFERER string = "referer"
	const AGENT string = "user-agent"

	//get the url we need to redirect to
	var redirectUrl Link
	routeId := req.PathParameters[ROUTE_ID]

	table := Connect()
	lookupErr := table.Get("PK", fmt.Sprintf("LINK#%s", routeId)).Range("SK", dynamo.BeginsWith, "USER#").One(&redirectUrl)

	//link visit is the information about the person visiting the link
	visit := &Visit{
		PK:          fmt.Sprintf("LINK#%s", routeId),
		SK:          fmt.Sprintf("VISIT#%s", fmt.Sprint(time.Now().UnixNano()/1e6)), //get unix time in ms
		IP:          req.Headers[IP],
		Language:    req.Headers[LANGUAGE],
		Referer:     req.Headers[REFERER],
		AgentHeader: req.Headers[AGENT],
		User:        redirectUrl.SK,
	}

	if lookupErr != nil {
		fmt.Println("LOOKUP ERR " + lookupErr.Error())
	}

	err := table.Put(visit).Run()

	if err != nil {
		fmt.Println("error:" + err.Error())
	}

	proxyRes := events.APIGatewayProxyResponse{
		Headers:    map[string]string{"location": redirectUrl.Url},
		StatusCode: 303,
	}

	return proxyRes, nil
}
