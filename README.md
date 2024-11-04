# Axiom Lambda Extension

Use Axiom Lambda Extension to send logs and platform events of your Lambda function to [Axiom](https://axiom.co/). Axiom detects the extension and provides you with quick filters and a dashboard.

With the Axiom Lambda extension, you can forget about the extra configuration of CloudWatch and subscription filters.

## Usage


```sh
aws lambda update-function-configuration --function-name my-function \
    --layers arn:aws:lambda:AWS_REGION:694952825951:layer:axiom-extension-ARCH:VERSION
```

## Documentation

For more information on how to set up and use the Axiom Lambda Extension, see the [Axiom documentation](https://axiom.co/docs/send-data/aws-lambda).
