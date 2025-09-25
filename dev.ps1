$uniqueSuffix = (Get-Date -UFormat %s)

$region = "us-east-2"
$s3BucketName = "whatsbot-flow-bucket-$uniqueSuffix"

# Table names
$userTableName = "UserState"
$historyTableName = "ConversationHistory"
$sessionTableName = "Session"
$s3FlowKey = "conversation.json"

# Helpers
function Log {
    param([string]$message)
    Write-Output $message
}

function Run {
    param([string]$command, [string[]]$arguments)
    Log "→ $command $($arguments -join ' ')"
    & $command @arguments *> "dev.log"

    # Check if command was successful
    if ($LASTEXITCODE -ne 0) {
        Log "Command failed with exit code: $LASTEXITCODE"
        Get-Content "dev.log" | Write-Output
        exit $LASTEXITCODE
    }
}

# DynamoDB
Log "Creating DynamoDB tables..."

Run "aws" @(
    "dynamodb", "create-table",
    "--table-name", $userTableName,
    "--attribute-definitions", "AttributeName=UserID,AttributeType=S",
    "--key-schema", "AttributeName=UserID,KeyType=HASH",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)

Run "aws" @(
    "dynamodb", "create-table",
    "--table-name", $historyTableName,
    "--attribute-definitions", "AttributeName=UserID,AttributeType=S", "AttributeName=Timestamp,AttributeType=N",
    "--key-schema", "AttributeName=UserID,KeyType=HASH", "AttributeName=Timestamp,KeyType=RANGE",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)

Run "aws" @(
    "dynamodb", "create-table",
    "--table-name", $sessionTableName,
    "--attribute-definitions", "AttributeName=SessionID,AttributeType=S",
    "--key-schema", "AttributeName=SessionID,KeyType=HASH",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)

Log "DynamoDB tables created"

# S3
Log "Creating S3 bucket: $s3BucketName"

# For regions except us-east-1, we need to specify the location constraint
Run "aws" @(
    "s3api", "create-bucket",
    "--bucket", $s3BucketName,
    "--region", $region,
    "--create-bucket-configuration", "LocationConstraint=$region"
)

Log "S3 bucket created"

# Wait a moment for bucket creation to propagate
Log "Waiting for bucket to be ready..."
Start-Sleep -Seconds 5

# Verify bucket exists before uploading
Log "Verifying bucket exists..."
Run "aws" @(
    "s3api", "head-bucket",
    "--bucket", $s3BucketName
)

Log "Uploading $s3FlowKey to bucket..."
Run "aws" @(
    "s3", "cp", ".\$s3FlowKey", "s3://$s3BucketName/$s3FlowKey"
)

Log "Upload complete"

# Quick summary
Log "----------------------------------"
Log "Resources created:"
Log "* S3 Bucket:       $s3BucketName"
Log "* DynamoDB Tables: $userTableName, $historyTableName, $sessionTableName"
Log "----------------------------------"

# .env file creation
$envFileContent = @"
# Environment for Whatsbot
BOT_LOG_LEVEL=DEBUG
BOT_FSM_S3_BUCKET=$s3BucketName
BOT_FSM_S3_KEY=$s3FlowKey
BOT_DYNAMODB_USER_TABLE=$userTableName
BOT_DYNAMODB_HISTORY_TABLE=$historyTableName
BOT_DYNAMODB_SESSION_TABLE=$sessionTableName
BOT_SESSION_ID=primary-bot-session
"@

$envPath = ".env"
Set-Content -Path $envPath -Value $envFileContent -Encoding UTF8
Log ".env file created at $envPath" "Green"
