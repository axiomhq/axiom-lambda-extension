# Axiom Lambda Extension


The axiom-extension can send the logs and platform events of your Lambda function to [Axiom](https://axiom.co/). Axiom will detect the extension and provide you with quick filters and a dashboard.


But by using the axiom Lambda extension, you can forget the extra configuration regarding cloudwatch and subscription filters.

**Note:** The Lambda Service will still sends the logs to CloudWatch logs. If You want to disable cloudwatch logging, you will need
to deny the Lambda Function access to cloudwatch by explicit IAM policies.


## Quickstart

1. Set these environment variables on your function:

   - `AXIOM_DATASET`: The dataset name to send logs to
   - `AXIOM_TOKEN`: The Axiom API token (needs ingest permission into the dataset above). learn more about creating token [here](https://www.axiom.co/docs/restapi/token#creating-an-access-token)


2. Add the extension as a layer with the AWS CLI:

```shell
$ aws lambda update-function-configuration --function-name my-function \
    --layers arn:aws:lambda:<AWS_REGION>:694952825951:layer:axiom-extension-<ARCH>:<VERSION>
```

<details>
<summary>
All Lambda Layers
</summary>

|  Region | arm64 | x86_64 |
|---------|--------|---------|
| us-west-1 | `arn:aws:lambda:us-west-1:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:us-west-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| us-west-2  | `arn:aws:lambda:us-west-2:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:us-west-2:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| us-east-1 | `arn:aws:lambda:us-east-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:us-east-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| us-east-2 | `arn:aws:lambda:us-east-2:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:us-east-2:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| eu-west-1 | `arn:aws:lambda:eu-west-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:eu-west-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| eu-west-2 | `arn:aws:lambda:eu-west-2:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:eu-west-2:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| eu-west-3  | `arn:aws:lambda:eu-west-3:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:eu-west-3:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| eu-north-1 | `arn:aws:lambda:eu-north-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:eu-north-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| eu-central-1 | `arn:aws:lambda:eu-central-1:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:eu-central-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ca-central-1 | `arn:aws:lambda:ca-central-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:ca-central-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| sa-east-1 | `arn:aws:lambda:sa-east-1:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:sa-east-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-south-1  | `arn:aws:lambda:ap-south-1:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:ap-south-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-southeast-1 | `arn:aws:lambda:ap-southeast-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:ap-southeast-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-southeast-2 | `arn:aws:lambda:ap-southeast-2:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:ap-southeast-2:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-northeast-1 | `arn:aws:lambda:ap-northeast-1:694952825951:layer:axiom-extension-arm64:<VERSION>` | `arn:aws:lambda:ap-northeast-1:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-northeast-2 | `arn:aws:lambda:ap-northeast-2:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:ap-northeast-2:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
| ap-northeast-3  | `arn:aws:lambda:ap-northeast-3:694952825951:layer:axiom-extension-arm64:<VERSION>` |  `arn:aws:lambda:ap-northeast-3:694952825951:layer:axiom-extension-x86_64:<VERSION>` |
</details>


## Terraform Example
You can also use terraform to hook up your lambda with axiom lambda layer using the following:
1. plain terraform code
```tf
resource "aws_lambda_function" "test_lambda" {
  filename      = "lambda_function_payload.zip"
  function_name = "lambda_function_name"
  role          = aws_iam_role.iam_for_lambda.arn
  handler       = "index.test"
  runtime       = "nodejs14.x"

  ephemeral_storage {
    size = 10240 # Min 512 MB and the Max 10240 MB
  }

  environment {
    variables = {
      AXIOM_TOKEN   = "axiom-token"
      AXIOM_DATASET = "axiom-dataset"
    }
  }

  layers = [
    "arn:aws:lambda:<AWS_REGION>:694952825951:layer:axiom-extension-<ARCH>:<VERSION>"
  ]
}
```

2. Using [AWS lambda module](https://registry.terraform.io/modules/terraform-aws-modules/lambda/aws/latest)
```tf
module "lambda_function" {
  source = "terraform-aws-modules/lambda/aws"

  function_name = "my-lambda1"
  description   = "My awesome lambda function"
  handler       = "index.lambda_handler"
  runtime       = "python3.8"

  source_path = "../src/lambda-function1"

  layers = [
    "arn:aws:lambda:<AWS_REGION>:694952825951:layer:axiom-extension-<ARCH>:<VERSION>"
  ]

  environment_variables = {
    AXIOM_TOKEN   = "axiom-token"
    AXIOM_DATASET = "axiom-dataset"
  }
}
```

## Troubleshooting
Double check that the API token has permission to ingest into the dataset. If that wasn't the issue, please check the function logs on the AWS console, the extension will log any errors with setup or ingest.

## License

&copy; Axiom, Inc., 2022

Distributed under MIT License (`The MIT License`).
