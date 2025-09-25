$uniqueSuffix = (Get-Date -UFormat %s)

$region = "us-east-2"
$s3BucketName = "whatsbot-flow-bucket-$uniqueSuffix"

# Table names
$userTableName = "UserState"
$historyTableName = "ConversationHistory"
$sessionTableName = "Session"
$s3FlowKey = "conversation.json"

### Helpers ###

function Log {
    param([string]$message, [string]$color = "Gray")
    Write-Output $message -ForegroundColor $color
}

function Run {
    param([string]$command, [string[]]$args)
    Log "→ $command $($args -join ' ')" "DarkGray"
    Start-Process -FilePath $command -ArgumentList $args -NoNewWindow `
        -RedirectStandardOutput "dev.log" -RedirectStandardError "dev.log" -Wait
}

### DynamoDB ###

Log "Creating DynamoDB tables..." "White"

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

Log "DynamoDB tables created" "Green"

### S3 ###

Log "Creating S3 bucket: $s3BucketName" "White"
Run "aws" @(
    "s3api", "create-bucket",
    "--bucket", $s3BucketName,
    "--region", $region
)

Log "S3 bucket created" "Green"

Log "Uploading $s3FlowKey to bucket..." "White"
Run "aws" @(
    "s3", "cp", ".\$s3FlowKey", "s3://$s3BucketName/$s3FlowKey"
)

Log "Upload complete" "Green"

### Summary ###

Log "----------------------------------" "DarkGray"
Log "Resources created:" "Yellow"
Log "* S3 Bucket:       $s3BucketName" "Yellow"
Log "* DynamoDB Tables: $userTableName, $historyTableName, $sessionTableName" "Yellow"
Log "----------------------------------" "DarkGray"

### .env ###

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
