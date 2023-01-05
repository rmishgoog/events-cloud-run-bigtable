package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/bigtable"
)

var (
	project  string
	instance string
	table    string
)

var pollution_codes_to_status = map[int]string{
	100: "SEVERE",
	101: "MODERATE",
	102: "LOW",
	103: "NIL",
}

type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data"`
		ID   string `json:"id"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// type PubSubMessage struct {
// 	Data []byte `json:"data"`
// }

type CurrentStatus struct {
	State          string
	County         string
	City           string
	PollutionIndex int
	Temperature    float64
	AirPressure    float64
	WeekOfYear     int
	Year           int
}

func addLocation(writer http.ResponseWriter, request *http.Request) {
	var message PubSubMessage
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Printf("Error invoking ioutil.ReadAll(), %v", err)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &message); err != nil {
		log.Printf("Error invoking json.Unmarshal(), %v", err)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err = write(project, instance, table, message); err != nil {
		log.Fatalf("function().addLocation: Error occurred while adding a new row %v", err)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func write(project, instance string, tableName string, message PubSubMessage) error {
	ctx := context.Background()
	client, err := bigtable.NewClient(ctx, project, instance)
	if err != nil {
		return fmt.Errorf("bigtable.NewClient: %v", err)
	}
	defer client.Close()
	table := client.Open(tableName)
	columnFamilyName := "climate_summary"
	timestamp := bigtable.Now()
	mut := bigtable.NewMutation()
	var currentStatus CurrentStatus
	if err := json.NewDecoder(bytes.NewBuffer(message.Message.Data)).Decode(&currentStatus); err != nil {
		return fmt.Errorf("Failed to unmarshall the payload %v", err)
	}

	state := currentStatus.State
	county := currentStatus.County
	city := currentStatus.City
	pollutionCode := currentStatus.PollutionIndex
	temperature := currentStatus.Temperature
	pressure := currentStatus.AirPressure
	week := currentStatus.WeekOfYear
	year := currentStatus.Year

	if _, found := pollution_codes_to_status[pollutionCode]; found == false {
		return fmt.Errorf("No valid pollution status description was found using the code %v", pollutionCode)
	}
	mut.Set(columnFamilyName, "pollution", timestamp, []byte(pollution_codes_to_status[pollutionCode]))
	mut.Set(columnFamilyName, "temperature", timestamp, []byte(fmt.Sprintf("%f", temperature)))
	mut.Set(columnFamilyName, "pressure", timestamp, []byte(fmt.Sprintf("%f", pressure)))

	rowKey := state + "#" + county + "#" + city + "#" + strconv.FormatInt(int64(year), 10) + "#" + strconv.FormatInt(int64(week), 10)
	if err := table.Apply(ctx, rowKey, mut); err != nil {
		return fmt.Errorf("BigTable write operation has failed with an error: %v", err)
	}

	return nil

}

func main() {
	project = os.Getenv("PROJECT")
	instance = os.Getenv("INSTANCE")
	table = os.Getenv("TABLE")
	port := os.Getenv("PORT")
	if project == "" || instance == "" || port == "" {
		log.Fatalf("Unable to start the service as one or more required fields is not supplied, project=%v, instnace=%v, table=%v", project, instance, table)
		return
	}
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", addLocation)
	log.Printf("The service will be listening on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Unable to start the service %v", err)
	}
}
