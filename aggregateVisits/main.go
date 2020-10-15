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

func updateVisitCounter(vc V.VisitCounter, l V.Location) V.VisitCounter {
	//if both country and state are already set, just increment the totals
	if _, ok := vc.Locations[l.Country][l.State]; ok {
		vc.Total += 1
		vc.Locations[l.Country][l.State] += 1
		vc.Locations[l.Country]["Total"] += 1
		//if just country is set, then add the state and increment the total
	} else if _, ok := vc.Locations[l.Country]; ok {
		vc.Total += 1
		vc.Locations[l.Country][l.State] = 1
		vc.Locations[l.Country]["Total"] += 1
		//if the country isnt set, set the country and the state
	} else {
		vc.Total += 1
		vc.Locations[l.Country] = map[string]int{
			"Total": 1,
			l.State: 1,
		}
	}
	return vc
}

func updateLanguageCounter(lc map[string]int, l string) map[string]int {
	//if the language is already in the map, increment it
	if _, ok := lc[l]; ok {
		lc[l] += 1
		//if its not already in the list, set it to one
	} else {
		lc[l] = 1
	}
	return lc
}

func updateRefererCounter(rc map[string]int, r string) map[string]int {

	fmt.Println("!!!!!!!!!!!!" + r)

	//if the language is already in the map, increment it
	if _, ok := rc[r]; ok {
		rc[r] += 1
		//if its not already in the list, set it to one
	} else {
		rc[r] = 1
	}
	return rc
}

func updateUserAgentCounter(uac V.UserAgentCounter, ua V.UserAgent) V.UserAgentCounter {
	//if the browser is already in the map, increment it
	fmt.Print("!!!!!!!!!!!!")
	fmt.Println(uac)
	if _, ok := uac.Browser[ua.Browser]; ok {
		uac.Browser[ua.Browser] += 1
		//if its not already in the list, set it to one
	} else {
		uac.Browser = map[string]int{
			ua.Browser: 1,
		}
	}
	//if the OS is already in the map, increment it
	if _, ok := uac.OS[ua.OS]; ok {
		uac.OS[ua.OS] += 1
		//if its not already in the list, set it to one
	} else {
		uac.OS = map[string]int{
			ua.OS: 1,
		}
	}
	//if the Platform is already in the map, increment it
	if _, ok := uac.Platform[ua.Platform]; ok {
		uac.Platform[ua.Platform] += 1
		//if its not already in the list, set it to one
	} else {
		uac.Platform = map[string]int{
			ua.Platform: 1,
		}
	}
	//increment the isMobile
	if uac.IsMobile == 0 && ua.IsMobile {
		uac.IsMobile = 1
	} else if ua.IsMobile {
		uac.IsMobile += 1
	}
	return uac
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

			v, marshallErr := dynamo.MarshalItem(visit)

			if marshallErr != nil {
				fmt.Print("Error with marshalling: ")
				fmt.Println(visit)
			}

			if marshallErr != nil {
				fmt.Print("Error with marshalling: ")
				fmt.Println(visit)
			}

			//update the VISIT item
			table.Update("PK", v["PK"]).Range("SK", v["SK"]).Set("AgentHeader", v["AgentHeader"]).Set("Location", v["Location"]).Set("IP", v["IP"]).Set("Language", v["Language"]).Set("UserAgent", v["UserAgent"]).Run()

			//get the LINK item in order to aggregate
			var visits V.Link
			table.Get("PK", v["PK"]).Range("SK", dynamo.Equal, v["User"]).One(&visits)

			fmt.Print("!!!!!!!!!!!!!!")
			fmt.Println(visits)

			vc := updateVisitCounter(visits.Visits, visit.Location)
			lc := updateLanguageCounter(visits.Languages, visit.Language)
			rc := updateRefererCounter(visits.Referers, visit.Referer)
			uac := updateUserAgentCounter(visits.UserAgents, visit.UserAgent)

			mvc, marshallErr := dynamo.MarshalItem(vc)
			mlc, _ := dynamo.MarshalItem(lc)
			mrc, _ := dynamo.MarshalItem(rc)
			muac, _ := dynamo.MarshalItem(uac)

			//update the LINK item with new aggregated data
			e := table.Update("PK", v["PK"]).Range("SK", v["User"]).Set("Visits", mvc).Set("Languages", mlc).Set("UserAgents", muac).Set("Referers", mrc).Run()

			if e != nil {
				fmt.Print("ERROR:")
				fmt.Println(e.Error())
			}

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
