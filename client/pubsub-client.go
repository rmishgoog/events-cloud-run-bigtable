package main

// [START pubsub_publish_custom_attributes]
import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
)

type PayLoad struct {
	State          string
	County         string
	City           string
	PollutionIndex int
	Temperature    float64
	AirPressure    float64
	WeekOfYear     int
	Year           int
}

// Change the payload for testing
func publishJsonData(projectID, topicID string) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	data := PayLoad{
		State:          "IL",
		County:         "Will",
		City:           "Bolingbrook",
		PollutionIndex: 100,
		Temperature:    21.6,
		AirPressure:    30,
		WeekOfYear:     2,
		Year:           2023,
	}
	if err != nil {
		return fmt.Errorf("Error occurred when obtaining a new client:%v", err)
	}
	defer client.Close()
	payload, _ := json.Marshal(data)
	fmt.Println(string(payload))
	t := client.Topic(topicID)
	result := t.Publish(ctx, &pubsub.Message{
		Data: []byte(payload),
	})
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("Error occurred when publishing the message and obtaining a new message id:%v", err)
	}
	fmt.Println("Published the message successfully with the id:", id)
	return nil

}

func main() {
	fmt.Printf("main(). Starting to publish the message")
	projectID := "<your-project-id"
	topicID := "us-climate-updates"
	//Please provide the project id and topic id above, uncomment the vars and you can then remove empty string declarations below
	//projectID := ""
	//topicID := "us-climate-updates"
	err := publishJsonData(projectID, topicID)
	if err != nil {
		fmt.Printf("Logging the error in main():%v", err)
	} else {
		fmt.Println("Published the message successfully")
	}
}
