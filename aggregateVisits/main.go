package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	V "linker/visits"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/guregu/dynamo"
	"github.com/mssola/user_agent"
)

func isVisitInsert(record events.DynamoDBEventRecord) bool {
	return record.EventName == "INSERT" && strings.HasPrefix(record.Change.NewImage["SK"].String(), "VISIT#")
}

func parseAgentString(agent string) V.UserAgent {
	ua := user_agent.New(agent)
	browser, _ := ua.Browser()

	return V.UserAgent{
		IsBot:    ua.Bot(),
		IsMobile: ua.Mobile(),
		Platform: ua.Platform(),
		OS:       ua.OS(),
		Browser:  browser,
	}
}

func Handler(ctx context.Context, e events.DynamoDBEvent) error {

	//connect to the table
	table := V.Connect()

	//pre-aggregate the data before conditionally writing to the LINK# object
	//var links map[string]V.Visit

	for _, record := range e.Records {

		if isVisitInsert(record) {

			visit, err := V.Create(record.Change.NewImage)

			if err != nil {
				fmt.Printf("Error parsing record: %s", err.Error())
				fmt.Println(record)
				return err
			}

			//first step is get an array of all the IPs to send for information about them
			//until i get the paid plan just pass them one at a time -.-'
			var location V.Location

			geoUrl := fmt.Sprintf("https://api.ipgeolocation.io/ipgeo?apiKey=%s&ip=%s&fields=geo&excludes=continent_code,continent_name,zipcode,latitude,longitude,city,district,country_code3", os.Getenv("IPGEO_API_KEU"), visit.IP)

			fmt.Println(geoUrl)

			resp, fetchErr := http.Get(geoUrl)

			if fetchErr != nil {
				fmt.Println("Error fetching IP information from: " + visit.SK)
			}

			defer resp.Body.Close()

			body, readErr := ioutil.ReadAll(resp.Body)
			if readErr != nil {
				log.Fatal(readErr)
			}

			//so now the location is stored in location
			json.Unmarshal(body, &location)

			//set the location attribute
			visit.Location = location

			//parse the header into something more readable
			visit.UserAgent = parseAgentString(visit.AgentHeader)

			fmt.Println(visit.UserAgent)

			//null the IP field
			visit.IP = ""

			//null the agent header field
			visit.AgentHeader = ""

			//set the language
			visit.Language = strings.Split(visit.Language, ",")[0]

			fmt.Print("Visit:")
			fmt.Println(visit)

			v, marshallErr := dynamo.MarshalItem(visit)

			if marshallErr != nil {
				fmt.Print("Error with marshalling: ")
				fmt.Println(visit)
			}

			table.Update("PK", v["PK"]).Range("SK", v["SK"]).Set("AgentHeader", v["AgentHeader"]).Set("Location", v["Location"]).Set("IP", v["IP"]).Set("Language", v["Language"]).Set("UserAgent", v["UserAgent"]).Run()
			//table.Update("PK", v["PK"]).Range("SK", v["USER"]).SetExpr("Views.locations.$ = if_not_exists(Views.locations.$, ?) + ?", location.Country, location.Country, 0, 1).Run()

		}
	}

	return nil
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
