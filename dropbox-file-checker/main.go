package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/satori/go.uuid"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

type dropboxItem struct {
	SettingName  string `json:"SettingName"`
	SettingValue string `json:"SettingValue"`
}

const awsRegion = "eu-west-1"

// getCursorFromDb tries to get the latest file cursor from
// DynamoDB. If no value exist in the DB it returns empty string
func getCursorFromDb() string {

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		panic(err)
	}

	svc := dynamodb.New(sess)

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("Dropbox"),
		Key: map[string]*dynamodb.AttributeValue{
			"SettingName": {
				S: aws.String("Cursor"),
			},
		},
	})

	if err != nil {
		panic(fmt.Sprintf("Error while accessing DynamoDB: %v", err))
	}

	item := dropboxItem{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)

	if err != nil {
		log.Print(fmt.Sprintf("Failed to unmarshal Record, %v", err))
		return ""
	}

	log.Print("Cursor from DynamoDB: ", item.SettingValue)

	if item.SettingValue == "empty" {
		return ""
	}

	return item.SettingValue

}

func getFilesFromDropboxWithCursor(cursor string) (string, []string, bool) {
	config := dropbox.Config{
		Token:    getDropboxAPIToken(),
		LogLevel: dropbox.LogOff,
	}

	dbx := files.New(config)

	lstFolderResult, err := dbx.ListFolderContinue(&files.ListFolderContinueArg{
		Cursor: cursor,
	})

	if err != nil {
		log.Print("getFilesFromDropboxWithCursor: error while calling dropbox api: ", err)
		panic(err)
	}

	var jpgFiles = make([]string, 0)

	for _, element := range lstFolderResult.Entries {
		// elemet could be FileMetaData folderMetaData or DeletedMetadata.
		// we only need to do sth about the fieles
		meta, ok := element.(*files.FileMetadata)

		if ok == true {
			if (strings.HasSuffix(strings.ToLower(meta.PathDisplay), "jpg")) || (strings.HasSuffix(strings.ToLower(meta.PathDisplay), "jpeg")) {
				jpgFiles = append(jpgFiles, meta.PathDisplay)
			}
		}
	}

	return lstFolderResult.Cursor, jpgFiles, lstFolderResult.HasMore
}

func getCursorAndFilesFromDropbox() (string, []string, bool) {

	config := dropbox.Config{
		Token:    getDropboxAPIToken(),
		LogLevel: dropbox.LogOff,
	}

	dbx := files.New(config)

	lstFolderResult, err := dbx.ListFolder(&files.ListFolderArg{
		Path:                            "",
		IncludeDeleted:                  false,
		IncludeHasExplicitSharedMembers: false,
		IncludeMediaInfo:                true,
		IncludeMountedFolders:           true,
		Recursive:                       true,
		Limit:                           5,
	})

	if err != nil {
		panic(fmt.Sprintf("Error while calling Dropbox API for getting a cursor: %v", err))
	}

	var jpgFiles = make([]string, 0)

	for _, element := range lstFolderResult.Entries {
		meta := element.(*files.FileMetadata)

		if (strings.HasSuffix(strings.ToLower(meta.PathDisplay), "jpg")) || (strings.HasSuffix(strings.ToLower(meta.PathDisplay), "jpeg")) {
			jpgFiles = append(jpgFiles, meta.PathDisplay)
		}
	}

	return lstFolderResult.Cursor, jpgFiles, lstFolderResult.HasMore
}

func getSecretValuefromAWS(region string, arn string, key string) string {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		panic(err)
	}
	svc := secretsmanager.New(sess)

	secretValue, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		panic(err)
	}

	/*
		value: {
			ARN: "arn:aws:secretsmanager:eu-west-1:677189920773:secret:prod/dropbox-api/dashboard-analyser/siebel.stefan@gmail.com-1Bomk6",
			CreatedDate: 2018-06-18 09:44:01 +0000 UTC,
			Name: "prod/dropbox-api/dashboard-analyser/siebel.stefan@gmail.com",
			SecretString: "{\"Dropbox-API-Access-Token\":\"fLdyTui_ljgAAAAAAAAl1I667hWj0XP_-gxfMfgcfxObEGmfsaULjZrGOCBgbhxE\"}",
			VersionId: "5be85f0d-7185-47ff-8fba-e3b23ecc2f62",
			VersionStages: ["AWSCURRENT"]
		  }
	*/
	var result map[string]interface{}
	json.Unmarshal([]byte(*secretValue.SecretString), &result)

	return result[key].(string)
}

// getDropboxAPIToken gets the Dropbox API token from AWS secret manager
func getDropboxAPIToken() string {
	return getSecretValuefromAWS(awsRegion,
		"arn:aws:secretsmanager:eu-west-1:677189920773:secret:prod/dropbox-api/dashboard-analyser/siebel.stefan@gmail.com-1Bomk6",
		"Dropbox-API-Access-Token")

}

// saveCursorToDyanomoDb saves the cursor that we got from dropbox API and saves it to the Dropbox table
func saveCursorToDynamoDb(cursor string) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		panic(err)
	}

	svc := dynamodb.New(sess)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":r": {
				S: aws.String(cursor),
			},
		},
		TableName: aws.String("Dropbox"),
		Key: map[string]*dynamodb.AttributeValue{
			"SettingName": {
				S: aws.String("Cursor"),
			},
		},
		ReturnValues:     aws.String("UPDATED_NEW"),
		UpdateExpression: aws.String("set SettingValue = :r"),
	}

	_, err = svc.UpdateItem(input)

	if err != nil {
		panic(fmt.Sprintf("Error while updating DynamoDB Dropbox Cursor: %v", err))
	}
}

func movesFiles(filename string) {

	log.Print("file start ", filename)

	// download file from dropbox
	config := dropbox.Config{
		Token:    getDropboxAPIToken(),
		LogLevel: dropbox.LogOff,
	}

	dbx := files.New(config)

	log.Print("Start download ", filename)
	_, fileBinary, err := dbx.Download(&files.DownloadArg{
		Path: filename,
	})
	if err != nil {
		log.Print("Error while downloading file from dropbox ", err)
		//panic(err)
		return
	}
	log.Print("End download ", filename)

	// save to s3
	sess, err1 := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err1 != nil {
		log.Print("error while creating new aws session ", err1)
		//panic(err1)
		return
	}

	svc := s3.New(sess)

	buffer, err2 := ioutil.ReadAll(fileBinary)

	fileBinary.Close()

	if err2 != nil {
		log.Print("Error while reading downloaded dropbox file ", err2)
		//panic(err2)
		return
	}

	guid, err := uuid.NewV4()

	if err != nil {
		log.Print("Error while generating a GUID")
	}

	newFilename := guid.String() + ".jpg"

	log.Print("Start Upload ", filename)
	_, err3 := svc.PutObject(&s3.PutObjectInput{
		Bucket:               aws.String("dashboard-analyser"),
		Key:                  aws.String(newFilename),
		ACL:                  aws.String("private"),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(int64(len(buffer))),
		ContentType:          aws.String(http.DetectContentType(buffer)),
		ContentDisposition:   aws.String("attachment"),
		ServerSideEncryption: aws.String("AES256"),
	})
	log.Print("end upload ", filename)

	if err3 != nil {
		log.Print("Error while uploading the file to s3 ", err3)
		//panic(err3)
		return
	}
	log.Print("file end ", filename)

}

// Handler gets called by main function. if this method is called it means a file in the dropbox app folder has changed.
// we don't know yet which files changed, so first we are going to call
// the /list_folder/continue endpoint to get the latest changes to the folder
// we loop over the files, move them to s3 and delete them on dropbox
func Handler() {

	// get cursor from dynamoDB
	cursor := getCursorFromDb()

	// if the cursor is empty it means we didn't have one in the DB
	// which means that this applicatin is running for the first time.
	// in this case we pull all jpg files and

	if cursor == "" {
		var jpgFiles []string
		// create waitgroup for async processing
		var wg sync.WaitGroup
		var hasMore bool

		cursor, jpgFiles, hasMore = getCursorAndFilesFromDropbox()

		for hasMore == true || len(jpgFiles) > 0 {
			wg.Add(len(jpgFiles))

			for i, item := range jpgFiles {

				log.Print("working on file ", i)
				go func(file string, w *sync.WaitGroup) {
					movesFiles(file)
					w.Done()
				}(item, &wg)

			}

			// reset array to 0 element for loop above
			jpgFiles = make([]string, 0)

			if hasMore == true {
				cursor, jpgFiles, hasMore = getFilesFromDropboxWithCursor(cursor)
			}
		}
		wg.Wait()
	} else {
		var jpgFiles []string
		// create waitgroup for async processing
		var wg sync.WaitGroup
		var hasMore bool

		cursor, jpgFiles, hasMore = getFilesFromDropboxWithCursor(cursor)

		for hasMore == true || len(jpgFiles) > 0 {
			wg.Add(len(jpgFiles))

			for i, item := range jpgFiles {

				log.Print("working on file ", i)
				go func(file string, w *sync.WaitGroup) {
					movesFiles(file)
					w.Done()
				}(item, &wg)

			}

			// reset array to 0 element for loop above
			jpgFiles = make([]string, 0)

			if hasMore == true {
				cursor, jpgFiles, hasMore = getFilesFromDropboxWithCursor(cursor)
			}
		}
		wg.Wait()
	}

	//saveCursorToDynamoDb(cursor)
	saveCursorToDynamoDb(cursor)

	log.Print("Cursor:", cursor)

	return
}

func main() {
	lambda.Start(Handler)
}
