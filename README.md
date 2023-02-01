# Axiom Lambda Extension

Axiom Lambda Extension Enables AWS Lambda Users to send AWS lambda functions logs to Axiom axiom.co.

Previous Way to send Lambda Logs to Axiom was by Using cloudwatch log group subscription filters to send the logs
to axiom.

But by using the axiom Lambda extension, you can forget the extra configuration regarding cloudwatch and subscription filters.

**Note:** The Lambda Service will still sends the logs to CloudWatch logs. If You want to disable cloudwatch logging, you will need
to deny the Lambda Function access to cloudwatch by explicit IAM policies.


## Usage
### Configuration
You need to add the following enivornment variables to your lambda:
1. `AXIOM_DATASET`: The axiom.co dataset to send the lambda function logs to.
2. `AXIOM_TOKEN`: The access token to authenticat to axiom.co. learn more about creating token [here](https://www.axiom.co/docs/restapi/token#creating-an-access-token)

### ClI
you can Update the lambda function with the axiom lambda extension layer by using the following command.
```
aws lambda update-code-configuration \
	--function-name <FUNCTION_NAME> \
	--layers "arn:aws:lambda:<AWS_REGION>:919601712473:layer:axiom-lambda-extension:<VERSION>"
```

## License

&copy; Axiom, Inc., 2022

Distributed under MIT License (`The MIT License`).
