# Dashboard Analyser

A small learning project utilizing Golang and AWS. Read more about it on my blog: https://stefansiebel.wordpress.com/2018/07/23/learning-by-doing-an-aws-project/

So far the project is in a state where is merely working. Most of the times at least... more work to do. So don't judge ...

## TODOs (lots of them :-) )
- Refactor code to become unit testable and add unit tests
- Implement Lambda to clean up files on Dropbox and S3
- add consistent logging
  - dashboard-analyser
  - dropbox-webhook
  - dropbox-file-checker
- Improve error handling
  - dashboard-analyser
  - dropbox-webhook
  - dropbox-file-checker
- Add scripts to automatically spin up and tear down AWS infrastructure
  - IAM Roles
  - Secrets
  - Lambda Functions
  - DynamoDB
  - SNS
  - S3
  - API Gateway
 - Add a simple UI to display the data from dynamoDB
