#!/bin/bash
# deploy.sh

# Variables
AWS_REGION="us-east-1"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
FUNCTION_NAME="chatbot-magally"
ECR_REPO="chatbot-magally-repo"

# 1. Crear repositorio ECR
aws ecr create-repository --repository-name $ECR_REPO --region $AWS_REGION || true

# 2. Login a ECR
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

# 3. Build y push de imagen
docker build -t $FUNCTION_NAME .
docker tag $FUNCTION_NAME:latest $ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO:latest
docker push $ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO:latest

# 4. Crear rol IAM para Lambda
aws iam create-role --role-name lambda-chatbot-role \
    --assume-role-policy-document '{
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": {"Service": "lambda.amazonaws.com"},
            "Action": "sts:AssumeRole"
        }]
    }' || true

# 5. Adjuntar políticas necesarias
aws iam attach-role-policy --role-name lambda-chatbot-role \
    --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Policy para DynamoDB y Bedrock
aws iam put-role-policy --role-name lambda-chatbot-role \
    --policy-name chatbot-permissions \
    --policy-document '{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "dynamodb:PutItem",
                    "dynamodb:GetItem",
                    "dynamodb:Query",
                    "dynamodb:UpdateItem"
                ],
                "Resource": "arn:aws:dynamodb:'$AWS_REGION':'$ACCOUNT_ID':table/chatbot-conversations"
            },
            {
                "Effect": "Allow",
                "Action": [
                    "bedrock:InvokeModel"
                ],
                "Resource": "*"
            }
        ]
    }'

# 6. Crear tabla DynamoDB
aws dynamodb create-table \
    --table-name chatbot-conversations \
    --attribute-definitions \
        AttributeName=phone_number,AttributeType=S \
        AttributeName=timestamp,AttributeType=N \
    --key-schema \
        AttributeName=phone_number,KeyType=HASH \
        AttributeName=timestamp,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    --region $AWS_REGION || true

# 7. Crear función Lambda
aws lambda create-function \
    --function-name $FUNCTION_NAME \
    --role arn:aws:iam::$ACCOUNT_ID:role/lambda-chatbot-role \
    --code ImageUri=$ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO:latest \
    --package-type Image \
    --timeout 30 \
    --memory-size 512 \
    --region $AWS_REGION || \
aws lambda update-function-code \
    --function-name $FUNCTION_NAME \
    --image-uri $ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO:latest

# 8. Crear API Gateway
API_ID=$(aws apigatewayv2 create-api \
    --name chatbot-magally-api \
    --protocol-type HTTP \
    --target arn:aws:lambda:$AWS_REGION:$ACCOUNT_ID:function:$FUNCTION_NAME \
    --query ApiId \
    --output text)

# 9. Dar permisos a API Gateway para invocar Lambda
aws lambda add-permission \
    --function-name $FUNCTION_NAME \
    --statement-id apigateway-invoke \
    --action lambda:InvokeFunction \
    --principal apigateway.amazonaws.com \
    --source-arn "arn:aws:execute-api:$AWS_REGION:$ACCOUNT_ID:$API_ID/*/*" || true

echo "API Gateway URL: https://$API_ID.execute-api.$AWS_REGION.amazonaws.com/"