package main

import (
	"context"
	"log"
	"strconv"

	"github.com/satori/go.uuid"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/rekognition"
)

const awsRegion = "eu-west-1"

type analysedImage struct {
	TotalKm          int    `json:"TotalKm"`
	Filename         string `json:"Filename"`
	DateImported     string `json:"Import-Date"`
	GUID             string `json:"GUID"`
	DeletedOnDropbox bool   `json:"DeletedOnDropbox"`
	DeletedOnS3      bool   `json:"DeletedOnS3"`
}

func getDetectedText(bucket string, filename string) []int {

	log.Print("Bucket: ", bucket)
	log.Print("Filename: ", filename)

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		panic(err)
	}

	svc := rekognition.New(sess)

	detectedText, rErr := svc.DetectText(&rekognition.DetectTextInput{
		Image: &rekognition.Image{
			S3Object: &rekognition.S3Object{
				Bucket: aws.String(bucket),
				Name:   aws.String(filename),
			},
		},
	})

	if rErr != nil {
		panic(rErr)
	}

	numbers := filterNumbersFromTextDetections(detectedText.TextDetections)

	log.Print(numbers)

	return numbers
}

func filterNumbersFromTextDetections(detections []*rekognition.TextDetection) []int {
	var numbers []int
	numbers = make([]int, 0)

	for _, detection := range detections {
		if *detection.Confidence > float64(90) {

			number, err := strconv.Atoi(*detection.DetectedText)

			if err == nil {
				numbers = append(numbers, number)
			}
		}
	}
	return numbers
}

func findMaxInt(numbers []int) int {
	max := -1

	for _, number := range numbers {
		if max < number {
			max = number
		}
	}

	return max
}

func saveTotalToDynamoDB(image analysedImage) {
	log.Print("save")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		panic(err)
	}

	svc := dynamodb.New(sess)

	av, err := dynamodbattribute.MarshalMap(image)

	if err != nil {
		panic(err)
	}

	_, err = svc.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String("Dashboard-Analyser"),
	})

	if err != nil {
		panic(err)
	}

	log.Print("image object: ", image)
	log.Print("dynamo db API Version: ", svc.APIVersion)

}

// Handler is the main function that is called by the lambda handler
func Handler(ctx context.Context, s3records events.S3Event) {

	for _, record := range s3records.Records {
		numbers := getDetectedText(record.S3.Bucket.Name, record.S3.Object.Key)
		log.Print("numbers found in image: ", numbers)
		totalKm := findMaxInt(numbers)
		log.Print("total km ", totalKm)
		if totalKm > 0 {
			u, err := uuid.NewV4()
			if err != nil {
				panic(err)
			}

			saveTotalToDynamoDB(analysedImage{
				TotalKm:          totalKm,
				Filename:         record.S3.Object.Key,
				DateImported:     record.EventTime.String(),
				GUID:             u.String(),
				DeletedOnDropbox: false,
				DeletedOnS3:      false,
			})
		}
	}

	return
}

func main() {
	lambda.Start(Handler)
}
