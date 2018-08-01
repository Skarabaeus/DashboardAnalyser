package main

import (
	"errors"

	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	// ErrNameNotProvided is thrown when a name is not provided
	ErrNameNotProvided = errors.New("no name was provided in the HTTP body")
)

// Handler is your Lambda function handler
// It uses Amazon API Gateway request/responses provided by the aws-lambda-go/events package,
// However you could use other event sources (S3, Kinesis etc), or JSON-decoded primitive types such as 'string'.
func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// checking if a query string parameter named "challenge" got sent over
	// if yes, just return it in the response body. This is for verifying the
	// dropbox webhook.
	if request.QueryStringParameters != nil {
		var challange = request.QueryStringParameters["challenge"]
		if challange != "" {
			var headers = map[string]string{
				"Content-Type":           "text/plain",
				"X-Content-Type-Options": "nosniff",
			}
			return events.APIGatewayProxyResponse{
				Body:            challange,
				StatusCode:      200,
				Headers:         headers,
				IsBase64Encoded: false,
			}, nil
		}
	} else {
		http.Get("https://u4byhq5rlj.execute-api.eu-west-1.amazonaws.com/prod/sns")
	}

	return events.APIGatewayProxyResponse{
		Body:            "ok",
		StatusCode:      200,
		Headers:         nil,
		IsBase64Encoded: false,
	}, nil

}

func main() {
	lambda.Start(Handler)
}
